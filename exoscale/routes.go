package exoscale

import (
	"context"
	"fmt"
	"strings"

	v3 "github.com/exoscale/egoscale/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

// Routes configuration (<-> cloud-config file)
type routesConfig struct {
	// Routes are opt-in: they require Nodes attached to a VPC Subnet and created with ip-forwarding.
	Enabled bool `yaml:"enabled"`
	VPC     string
	// Subnet the Nodes are attached to: Node VPC addresses are allocated here.
	Subnet string
	// Other Subnets whose instances must reach Pods directly. Return traffic is routed by the
	// replying instance's Subnet, so every such Subnet needs the Pod routes too.
	AdditionalSubnets []string `yaml:"additionalSubnets"`
}

type routes struct {
	p   *cloudProvider
	cfg *routesConfig
}

func newRoutes(provider *cloudProvider, config *routesConfig) cloudprovider.Routes {
	return &routes{
		p:   provider,
		cfg: config,
	}
}

// routeDescriptionPrefix marks the routes owned by a cluster, so that routes created by
// anything else (or by another cluster sharing the Subnet) are never listed nor deleted.
func routeDescriptionPrefix(clusterName string) string {
	return "k8s/" + clusterName + "/"
}

func routeTarget(ip string) string {
	return "ip=" + ip
}

func (r *routes) subnets() []v3.UUID {
	subnets := []v3.UUID{v3.UUID(r.cfg.Subnet)}
	for _, s := range r.cfg.AdditionalSubnets {
		subnets = append(subnets, v3.UUID(s))
	}
	return subnets
}

// ownedRoutes returns the cluster routes of a Subnet.
func (r *routes) ownedRoutes(ctx context.Context, clusterName string, subnet v3.UUID) ([]v3.ListRouteEntry, error) {
	res, err := r.p.client.ListRoutes(ctx, v3.UUID(r.cfg.VPC), subnet)
	if err != nil {
		return nil, fmt.Errorf("list routes of Subnet %s: %w", subnet, err)
	}

	var owned []v3.ListRouteEntry
	for _, route := range res.Routes {
		if strings.HasPrefix(route.Description, routeDescriptionPrefix(clusterName)) {
			owned = append(owned, route)
		}
	}
	return owned, nil
}

// subnetAddresses returns the VPC address of every instance attached to the Nodes Subnet.
func (r *routes) subnetAddresses(ctx context.Context) (map[v3.UUID]string, error) {
	subnet, err := r.p.client.GetSubnet(ctx, v3.UUID(r.cfg.VPC), v3.UUID(r.cfg.Subnet))
	if err != nil {
		return nil, fmt.Errorf("get Subnet %s: %w", r.cfg.Subnet, err)
	}

	addresses := make(map[v3.UUID]string, len(subnet.Instances))
	for _, i := range subnet.Instances {
		addresses[i.ID] = i.Ipv4.String()
	}
	return addresses, nil
}

// ListRoutes lists all managed routes that belong to the specified clusterName.
// A route missing from any Subnet, or pointing at different targets across Subnets, is left
// out: the route controller then calls CreateRoute again, which repairs it.
func (r *routes) ListRoutes(ctx context.Context, clusterName string) ([]*cloudprovider.Route, error) {
	addresses, err := r.subnetAddresses(ctx)
	if err != nil {
		return nil, err
	}

	nodes, err := r.p.kclient.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list Nodes: %w", err)
	}
	nodeByTarget := make(map[string]types.NodeName)
	for _, node := range nodes.Items {
		id, err := formatProviderID(node.Spec.ProviderID)
		if err != nil {
			continue
		}
		if ip, ok := addresses[id]; ok {
			nodeByTarget[routeTarget(ip)] = types.NodeName(node.Name)
		}
	}

	subnets := r.subnets()
	targets := make(map[string]map[string]struct{}) // destination -> set of targets
	seen := make(map[string]int)                    // destination -> Subnets it appears in
	for _, subnet := range subnets {
		owned, err := r.ownedRoutes(ctx, clusterName, subnet)
		if err != nil {
			return nil, err
		}
		for _, route := range owned {
			if targets[route.Destination] == nil {
				targets[route.Destination] = make(map[string]struct{})
			}
			targets[route.Destination][route.Target] = struct{}{}
			seen[route.Destination]++
		}
	}

	var list []*cloudprovider.Route
	for destination, set := range targets {
		for target := range set {
			node, ok := nodeByTarget[target]
			if !ok {
				// Target isn't a Node anymore: the route controller deletes it (see DeleteRoute).
				list = append(list, &cloudprovider.Route{
					Name:            target,
					DestinationCIDR: destination,
					Blackhole:       true,
				})
				continue
			}
			if len(set) > 1 || seen[destination] != len(subnets) {
				continue
			}
			list = append(list, &cloudprovider.Route{
				Name:            destination,
				TargetNode:      node,
				DestinationCIDR: destination,
			})
		}
	}
	return list, nil
}

// CreateRoute creates the described managed route in every configured Subnet, replacing
// any cluster route for the same destination pointing elsewhere.
// The Node instance is attached to the Nodes Subnet if needed.
func (r *routes) CreateRoute(ctx context.Context, clusterName string, _ string, route *cloudprovider.Route) error {
	node, err := r.p.kclient.CoreV1().Nodes().Get(ctx, string(route.TargetNode), metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get Node %s: %w", route.TargetNode, err)
	}
	id, err := formatProviderID(node.Spec.ProviderID)
	if err != nil {
		return fmt.Errorf("node %s: %w", route.TargetNode, err)
	}

	instance, err := r.p.client.GetInstance(ctx, id)
	if err != nil {
		return fmt.Errorf("get instance %s: %w", id, err)
	}
	// Without ip-forwarding, the platform anti-spoofing drops every Pod-sourced packet:
	// fail loudly here rather than create a route that silently blackholes.
	if instance.IPForwarding == nil || !*instance.IPForwarding {
		return fmt.Errorf("instance %s (Node %s) was not created with ip-forwarding", id, route.TargetNode)
	}

	ip, err := r.attachInstance(ctx, id)
	if err != nil {
		return err
	}

	for _, subnet := range r.subnets() {
		owned, err := r.ownedRoutes(ctx, clusterName, subnet)
		if err != nil {
			return err
		}

		exists := false
		for _, existing := range owned {
			if existing.Destination != route.DestinationCIDR {
				continue
			}
			if existing.Target == routeTarget(ip) {
				exists = true
				continue
			}
			if err := r.p.client.DeleteRoute(ctx, v3.UUID(r.cfg.VPC), subnet, existing.ID); err != nil {
				return fmt.Errorf("delete stale route %s of Subnet %s: %w", existing.ID, subnet, err)
			}
		}
		if exists {
			continue
		}

		if _, err := r.p.client.CreateRoute(ctx, v3.UUID(r.cfg.VPC), subnet, v3.CreateRouteRequest{
			Description: routeDescriptionPrefix(clusterName) + string(route.TargetNode),
			Destination: route.DestinationCIDR,
			Target:      routeTarget(ip),
		}); err != nil {
			return fmt.Errorf("create route %s in Subnet %s: %w", route.DestinationCIDR, subnet, err)
		}
		infof("created route %s -> %s (Node %s) in Subnet %s", route.DestinationCIDR, ip, route.TargetNode, subnet)
	}
	return nil
}

// attachInstance returns the instance address in the Nodes Subnet, attaching it first if needed.
func (r *routes) attachInstance(ctx context.Context, id v3.UUID) (string, error) {
	addresses, err := r.subnetAddresses(ctx)
	if err != nil {
		return "", err
	}
	if ip, ok := addresses[id]; ok {
		return ip, nil
	}

	op, err := r.p.client.AttachInstanceToSubnet(ctx, v3.UUID(r.cfg.VPC), v3.UUID(r.cfg.Subnet), v3.AttachInstanceToSubnetRequest{
		Instance: &v3.InstanceRef{ID: id},
	})
	if err != nil {
		return "", fmt.Errorf("attach instance %s to Subnet %s: %w", id, r.cfg.Subnet, err)
	}
	if _, err := r.p.client.Wait(ctx, op, v3.OperationStateSuccess); err != nil {
		return "", fmt.Errorf("attach instance %s to Subnet %s: %w", id, r.cfg.Subnet, err)
	}
	infof("attached instance %s to Subnet %s", id, r.cfg.Subnet)

	addresses, err = r.subnetAddresses(ctx)
	if err != nil {
		return "", err
	}
	ip, ok := addresses[id]
	if !ok {
		return "", fmt.Errorf("instance %s attached but has no address in Subnet %s", id, r.cfg.Subnet)
	}
	return ip, nil
}

// DeleteRoute deletes the cluster routes for the route destination from every configured Subnet.
// A blackhole route only deletes its own stale target, named after it by ListRoutes.
func (r *routes) DeleteRoute(ctx context.Context, clusterName string, route *cloudprovider.Route) error {
	for _, subnet := range r.subnets() {
		owned, err := r.ownedRoutes(ctx, clusterName, subnet)
		if err != nil {
			return err
		}
		for _, existing := range owned {
			if existing.Destination != route.DestinationCIDR || (route.Blackhole && existing.Target != route.Name) {
				continue
			}
			if err := r.p.client.DeleteRoute(ctx, v3.UUID(r.cfg.VPC), subnet, existing.ID); err != nil {
				return fmt.Errorf("delete route %s of Subnet %s: %w", existing.ID, subnet, err)
			}
			infof("deleted route %s -> %s in Subnet %s", existing.Destination, existing.Target, subnet)
		}
	}
	return nil
}

func (c *refreshableExoscaleClient) GetSubnet(ctx context.Context, vpcID v3.UUID, id v3.UUID) (*v3.Subnet, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetSubnet(
		ctx,
		vpcID,
		id,
	)
}

func (c *refreshableExoscaleClient) AttachInstanceToSubnet(
	ctx context.Context,
	vpcID v3.UUID,
	subnetID v3.UUID,
	req v3.AttachInstanceToSubnetRequest,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.AttachInstanceToSubnet(
		ctx,
		vpcID,
		subnetID,
		req,
	)
}

func (c *refreshableExoscaleClient) ListRoutes(ctx context.Context, vpcID v3.UUID, subnetID v3.UUID) (*v3.ListRoutesResponse, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.ListRoutes(
		ctx,
		vpcID,
		subnetID,
	)
}

func (c *refreshableExoscaleClient) CreateRoute(
	ctx context.Context,
	vpcID v3.UUID,
	subnetID v3.UUID,
	req v3.CreateRouteRequest,
) (*v3.Route, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.CreateRoute(
		ctx,
		vpcID,
		subnetID,
		req,
	)
}

func (c *refreshableExoscaleClient) DeleteRoute(ctx context.Context, vpcID v3.UUID, subnetID v3.UUID, id v3.UUID) error {
	c.RLock()
	defer c.RUnlock()

	return c.exo.DeleteRoute(
		ctx,
		vpcID,
		subnetID,
		id,
	)
}
