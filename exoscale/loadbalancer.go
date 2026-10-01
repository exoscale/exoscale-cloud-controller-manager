package exoscale

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"

	v3 "github.com/exoscale/egoscale/v3"
)

const (
	annotationPrefix                                 = "service.beta.kubernetes.io/exoscale-loadbalancer-"
	annotationLoadBalancerID                         = annotationPrefix + "id"
	annotationLoadBalancerName                       = annotationPrefix + "name"
	annotationLoadBalancerDescription                = annotationPrefix + "description"
	annotationLoadBalancerExternal                   = annotationPrefix + "external"
	annotationLoadBalancerServiceStrategy            = annotationPrefix + "service-strategy"
	annotationLoadBalancerServiceName                = annotationPrefix + "service-name"
	annotationLoadBalancerServiceDescription         = annotationPrefix + "service-description"
	annotationLoadBalancerServiceInstancePoolID      = annotationPrefix + "service-instancepool-id"
	annotationLoadBalancerSKSClusterName             = annotationPrefix + "sks-cluster-name" // required for annotationLoadBalancerServiceSKSNodePoolName
	annotationLoadBalancerServiceSKSNodePoolName     = annotationPrefix + "service-sks-nodepool-name"
	annotationLoadBalancerServiceHealthCheckMode     = annotationPrefix + "service-healthcheck-mode"
	annotationLoadBalancerServiceHealthCheckPort     = annotationPrefix + "service-healthcheck-port"
	annotationLoadBalancerServiceHealthCheckURI      = annotationPrefix + "service-healthcheck-uri"
	annotationLoadBalancerServiceHealthCheckInterval = annotationPrefix + "service-healthcheck-interval"
	annotationLoadBalancerServiceHealthCheckTimeout  = annotationPrefix + "service-healthcheck-timeout"
	annotationLoadBalancerServiceHealthCheckRetries  = annotationPrefix + "service-healthcheck-retries"
	annotationLoadBalancerIPAddressType              = annotationPrefix + "ip-address-type"
	annotationLoadBalancerIPv6ID                     = annotationPrefix + "ipv6-id"
	annotationLoadBalancerIPv6Name                   = annotationPrefix + "ipv6-name"
	annotationLoadBalancerIPv6TargetPorts            = annotationPrefix + "ipv6-target-ports"
	annotationLoadBalancerIPv6HealthCheckPort        = annotationPrefix + "ipv6-healthcheck-port"
)

const (
	ipAddressTypeIPv4      = "ipv4"
	ipAddressTypeIPv6      = "ipv6"
	ipAddressTypeDualStack = "dualstack"
)

var (
	defaultNLBServiceHealthCheckTimeout                                        = "5s"
	defaultNLBServiceHealthcheckInterval                                       = "10s"
	defaultNLBServiceHealthcheckMode     v3.LoadBalancerServiceHealthcheckMode = v3.LoadBalancerServiceHealthcheckModeTCP
	defaultNLBServiceHealthcheckRetries  int64                                 = 1
	defaultNLBServiceStrategy            v3.LoadBalancerServiceStrategy        = v3.LoadBalancerServiceStrategyRoundRobin
)

var errLoadBalancerNotFound = errors.New("load balancer not found")
var errLoadBalancerIDAnnotationNotFound = errors.New("load balancer ID annotation not found")
var errIPv6LoadBalancerIDAnnotationNotFound = errors.New("IPv6 load balancer ID annotation not found")

type loadBalancer struct {
	p   *cloudProvider
	cfg *loadBalancerConfig
}

// isExternal returns true if the NLB instance is marked as "external" in the
// Kubernetes Service manifest annotations (i.e. not managed by the CCM).
func (l loadBalancer) isExternal(service *v1.Service) bool {
	return strings.ToLower(getAnnotation(service, annotationLoadBalancerExternal, "false")) == "true"
}

func newLoadBalancer(provider *cloudProvider, config *loadBalancerConfig) cloudprovider.LoadBalancer {
	return &loadBalancer{
		p:   provider,
		cfg: config,
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) GetLoadBalancer(
	ctx context.Context,
	_ string,
	service *v1.Service,
) (*v1.LoadBalancerStatus, bool, error) {
	nlb, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	nlbIPv6, err := l.fetchIPv6LoadBalancer(ctx, service)
	if err != nil && !errors.Is(err, errLoadBalancerNotFound) {
		return nil, false, err
	}

	return loadBalancerStatus(nlb, nlbIPv6), true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancer) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID))
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one.
// Returns the status of the balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancer(
	ctx context.Context,
	_ string,
	service *v1.Service,
	nodes []*v1.Node,
) (*v1.LoadBalancerStatus, error) {
	ipAddressType, err := getIPAddressType(service)
	if err != nil {
		return nil, err
	}

	if ipAddressType == ipAddressTypeDualStack {
		if _, err := parseIPv6TargetPorts(service); err != nil {
			return nil, err
		}

		if l.isExternal(service) &&
			getAnnotation(service, annotationLoadBalancerIPv6ID, "") == "" &&
			getAnnotation(service, annotationLoadBalancerIPv6Name, "") == "" {
			return nil, errors.New(
				"NLB instance marked as external in Service annotations with dualstack IP address type, " +
					"but no IPv6 NLB ID or name specified",
			)
		}
	}

	if l.isExternal(service) {
		lbID := getAnnotation(service, annotationLoadBalancerID, "")
		lbName := getAnnotation(service, annotationLoadBalancerName, "")

		if lbID == "" && lbName == "" {
			return nil, errors.New("NLB instance marked as external in Service annotations, but no ID or name specified")
		}

		// If yet no NLB ID specified OR determined by a previous EnsureLoadBalancer run
		if lbID == "" && lbName != "" {
			nlbs, err := l.p.client.ListLoadBalancers(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing NLBs: %w", err)
			}

			nlb, err := nlbs.FindLoadBalancer(lbName)
			if err != nil {
				return nil, fmt.Errorf(
					"NLB instance is marked external by name %q, but no matching NLB was found",
					lbName,
				)
			}

			if err := l.patchAnnotation(ctx, service, annotationLoadBalancerID, nlb.ID.String()); err != nil {
				return nil, fmt.Errorf("error patching annotations: %w", err)
			}
			infof("found external NLB %q by name %q and patched ID %s", nlb.Name, lbName, nlb.ID)
		}
	}

	// Check if the annotationLoadBalancerSKSClusterName and annotationLoadBalancerServiceSKSNodePoolName exist
	if sksClusterName := getAnnotation(service, annotationLoadBalancerSKSClusterName, ""); sksClusterName != "" {
		if sksNodePoolName := getAnnotation(service, annotationLoadBalancerServiceSKSNodePoolName, ""); sksNodePoolName != "" {
			debugf("SKS Cluster name specified in Service annotations: %s", sksClusterName)
			debugf("SKS Node Pool name specified in Service annotations: %s", sksNodePoolName)

			// Get the list of SKS clusters
			sksClusters, err := l.p.client.ListSKSClusters(ctx)
			if err != nil {
				return nil, fmt.Errorf("error listing SKS clusters: %s", err)
			}

			sksCluster, err := sksClusters.FindSKSCluster(sksClusterName)
			if err != nil {
				return nil, fmt.Errorf("SKS cluster with name %s not found", sksClusterName)
			}

			// Find the SKS node pool ID by name
			var instancePoolID v3.UUID
			for _, pool := range sksCluster.Nodepools {
				if strings.EqualFold(pool.Name, sksNodePoolName) && pool.InstancePool != nil {
					instancePoolID = pool.InstancePool.ID
					break
				}
			}

			if instancePoolID == "" {
				return nil, fmt.Errorf("SKS node pool with name %s not found", sksNodePoolName)
			}

			debugf("inferred NLB service Instance Pool ID from SKS node pool name: %s", instancePoolID)

			err = l.patchAnnotation(ctx, service, annotationLoadBalancerServiceInstancePoolID, instancePoolID.String())
			if err != nil {
				return nil, fmt.Errorf("error patching annotations: %s", err)
			}
		}
	} else if getAnnotation(service, annotationLoadBalancerServiceSKSNodePoolName, "") != "" {
		return nil, errors.New("SKS node pool name specified without SKS cluster name")
	} else if getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, "") == "" {
		// Inferring the Instance Pool ID from the cluster Nodes that run the Service in case no Instance Pool ID
		// has been specified in the annotations.
		//
		// IMPORTANT: this use case is not compatible with Services referencing Pods using Node Selectors
		// (see https://github.com/kubernetes/kubernetes/issues/45234 for an explanation of the problem).
		// The list of Nodes passed as argument to this method contains *ALL* the Nodes in the cluster, not only the
		// ones that actually host the Pods targeted by the Service.

		debugf("no NLB service Instance Pool ID specified in Service annotations, inferring from cluster Nodes")

		var instancePoolID v3.UUID
		for _, node := range nodes {
			instance, err := l.p.client.GetInstance(ctx, v3.UUID(node.Status.NodeInfo.SystemUUID))
			if err != nil {
				return nil, fmt.Errorf("error retrieving Compute instance information: %s", err)
			}

			// Standalone Node, leaving it alone.
			if instance.Manager == nil || instance.Manager.Type != "instance-pool" {
				continue
			}

			if instancePoolID != "" && instance.Manager.ID != instancePoolID {
				return nil, errors.New(
					"multiple Instance Pools detected across cluster Nodes, " +
						"an Instance Pool ID must be specified in Service manifest annotations",
				)
			}

			instancePoolID = instance.Manager.ID
		}

		if instancePoolID == "" {
			return nil, errors.New("couldn't infer any Instance Pool from cluster Nodes")
		}

		debugf("inferred NLB service Instance Pool ID from cluster Nodes: %s", instancePoolID)

		err := l.patchAnnotation(ctx, service, annotationLoadBalancerServiceInstancePoolID, instancePoolID.String())
		if err != nil {
			return nil, fmt.Errorf("error patching annotations: %s", err)
		}
	}

	lbSpec, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return nil, err
	}

	if ipAddressType == ipAddressTypeDualStack {
		if err := l.checkInstancePoolDualStack(ctx, service, nodes); err != nil {
			return nil, err
		}
	}

	nlb, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if errors.Is(err, errLoadBalancerNotFound) {
			if l.isExternal(service) {
				return nil, errors.New("NLB instance marked as external in Service annotations, cannot create")
			}

			infof("creating new NLB %q", lbSpec.Name)

			op, err := l.p.client.CreateLoadBalancer(ctx, v3.CreateLoadBalancerRequest{
				Name:        lbSpec.Name,
				Description: lbSpec.Description,
				Labels:      lbSpec.Labels,
			})
			if err != nil {
				return nil, err
			}

			nlb, err = l.p.client.GetLoadBalancer(ctx, op.Reference.ID)
			if err != nil {
				return nil, err
			}

			if err := l.patchAnnotation(ctx, service, annotationLoadBalancerID, nlb.ID.String()); err != nil {
				return nil, fmt.Errorf("error patching annotations: %s", err)
			}

			debugf("NLB %q created successfully (ID: %s)", nlb.Name, nlb.ID)
		} else {
			return nil, err
		}
	}

	var nlbIPv6 *v3.LoadBalancer
	if ipAddressType == ipAddressTypeDualStack {
		if nlbIPv6, err = l.ensureIPv6LoadBalancer(ctx, service); err != nil {
			return nil, err
		}
	} else if getAnnotation(service, annotationLoadBalancerIPv6ID, "") != "" {
		// The Service went back to IPv4 only: release the IPv6 NLB.
		if err := l.ensureIPv6LoadBalancerDeleted(ctx, service); err != nil {
			return nil, err
		}
	}

	if err = l.updateLoadBalancer(ctx, service); err != nil {
		return nil, err
	}

	return loadBalancerStatus(nlb, nlbIPv6), nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, _ []*v1.Node) error {
	return l.updateLoadBalancer(ctx, service)
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still lying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	nlbIPv6, err := l.fetchIPv6LoadBalancer(ctx, service)
	switch {
	case err == nil:
		if err := l.deleteLoadBalancerServices(ctx, service, nlbIPv6); err != nil {
			return err
		}
	case !errors.Is(err, errLoadBalancerNotFound):
		return err
	}

	nlb, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if errors.Is(err, errLoadBalancerNotFound) {
			return nil
		}

		return err
	}

	return l.deleteLoadBalancerServices(ctx, service, nlb)
}

// deleteLoadBalancerServices deletes the NLB services matching the k8s Service ports,
// then the NLB instance itself if no other NLB service remains and it is not external.
func (l *loadBalancer) deleteLoadBalancerServices(ctx context.Context, service *v1.Service, nlb *v3.LoadBalancer) error {
	// Since a NLB instance can be shared among unrelated k8s Services,
	// as a safety precaution we delete the NLB services matching this k8s
	// Service's ports individually rather than the whole NLB instance directly.
	// If at the end of the process there are no unrelated NLB services
	// remaining, we can safely delete the NLB instance.
	remainingServices := len(nlb.Services)
	for _, nlbService := range nlb.Services {
		for _, servicePort := range service.Spec.Ports {
			if nlbService.Port == int64(servicePort.Port) && strings.EqualFold(string(nlbService.Protocol), string(servicePort.Protocol)) {
				infof("deleting NLB service %s/%s", nlb.Name, nlbService.Name)
				_, err := l.p.client.DeleteLoadBalancerService(ctx, nlb.ID, nlbService.ID)
				if err != nil {
					return err
				}

				remainingServices--
			}
		}
	}

	if remainingServices == 0 {
		if l.isExternal(service) {
			debugf("NLB instance marked as external in Service annotations, skipping delete")
			return nil
		}

		infof("deleting NLB %q", nlb.Name)

		_, err := l.p.client.DeleteLoadBalancer(ctx, nlb.ID)
		return err
	}

	return nil
}

// updateLoadBalancer updates the matching Exoscale NLB instances according to the *v1.Service spec provided.
// Both NLB instances are fetched and checked before any change, so that an address family conflict
// (e.g. an external NLB shared with another Service) doesn't leave a half-reconciled Service.
func (l *loadBalancer) updateLoadBalancer(ctx context.Context, service *v1.Service) error {
	ipAddressType, err := getIPAddressType(service)
	if err != nil {
		return err
	}

	nlbUpdate, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return err
	}

	if nlbUpdate.ID == "" {
		return errLoadBalancerIDAnnotationNotFound
	}

	nlbCurrent, err := l.p.client.GetLoadBalancer(ctx, nlbUpdate.ID)
	if err != nil {
		return err
	}

	if err := checkAddressFamily(nlbCurrent, v3.LoadBalancerAddressfamilyInet4); err != nil {
		return err
	}

	var nlbIPv6Update, nlbIPv6Current *v3.LoadBalancer
	if ipAddressType == ipAddressTypeDualStack {
		if nlbIPv6Update, err = buildIPv6LoadBalancerFromAnnotations(service); err != nil {
			return err
		}

		if nlbIPv6Update.ID == "" {
			return errIPv6LoadBalancerIDAnnotationNotFound
		}

		if nlbIPv6Current, err = l.p.client.GetLoadBalancer(ctx, nlbIPv6Update.ID); err != nil {
			return err
		}

		if err := checkAddressFamily(nlbIPv6Current, v3.LoadBalancerAddressfamilyInet6); err != nil {
			return err
		}
	}

	if err := l.reconcileLoadBalancer(ctx, service, nlbCurrent, nlbUpdate); err != nil {
		return err
	}

	if nlbIPv6Current != nil {
		return l.reconcileLoadBalancer(ctx, service, nlbIPv6Current, nlbIPv6Update)
	}

	return nil
}

// reconcileLoadBalancer updates the Exoscale NLB instance nlbCurrent and its services to match nlbUpdate.
func (l *loadBalancer) reconcileLoadBalancer(
	ctx context.Context,
	service *v1.Service,
	nlbCurrent *v3.LoadBalancer,
	nlbUpdate *v3.LoadBalancer,
) error {
	var err error

	// If this NLB is not marked as external and top-level fields changed, update them.
	if !l.isExternal(service) && isLoadBalancerUpdated(nlbCurrent, nlbUpdate) {
		infof("updating NLB %q", nlbCurrent.Name)

		if _, err = l.p.client.UpdateLoadBalancer(ctx, nlbUpdate.ID, v3.UpdateLoadBalancerRequest{
			Name:        nlbUpdate.Name,
			Description: nlbCurrent.Description,
			Labels:      nlbUpdate.Labels,
		}); err != nil {
			return err
		}

		debugf("NLB %q updated successfully", nlbCurrent.Name)
	}

	// First loop: delete any old NLB services whose port/protocol no longer exist in the updated spec.
	// Info: There is a long standing bug in kubectl where patching a Service towards
	// the same port tcp/udp and possible even other properties doesn't trigger
	// It needs then a server side apply or replace
	// kubectl apply --server-side
	// https://github.com/kubernetes/kubernetes/issues/39188
	// https://github.com/kubernetes/kubernetes/issues/105610
	type ServiceKey struct {
		Port     int64
		Protocol v3.LoadBalancerServiceProtocol
	}

	// We'll collect existing services that still match a port/protocol into this map
	nlbServices := make(map[ServiceKey]v3.LoadBalancerService)

next:
	for _, nlbServiceCurrent := range nlbCurrent.Services {
		key := ServiceKey{Port: nlbServiceCurrent.Port, Protocol: nlbServiceCurrent.Protocol}
		debugf("Checking existing NLB service %s/%s - key %v",
			nlbCurrent.Name, nlbServiceCurrent.Name, key)

		// See if there's a matching port/protocol in nlbUpdate
		for _, nlbServiceUpdate := range nlbUpdate.Services {
			updateKey := ServiceKey{Port: nlbServiceUpdate.Port, Protocol: nlbServiceUpdate.Protocol}

			if key == updateKey {
				// Keep it around for the second loop (updates)
				debugf("Match found for existing service %s/%s with updated service %s/%s",
					nlbCurrent.Name, nlbServiceCurrent.Name, nlbUpdate.Name, nlbServiceUpdate.Name)
				nlbServices[key] = nlbServiceCurrent

				continue next
			}
		}

		// If we got here, this existing NLB service doesn't match any desired port/protocol.
		if l.isExternal(service) {
			debugf(
				"NLB service %s/%s doesn't match any service port, but the NLB is marked external. "+
					"Avoiding deletion since it may belong to another Service.",
				nlbCurrent.Name,
				nlbServiceCurrent.Name,
			)
			continue
		}

		infof("NLB service %s/%s doesn't match any service port, deleting",
			nlbCurrent.Name,
			nlbServiceCurrent.Name)

		if _, err := l.p.client.DeleteLoadBalancerService(
			ctx,
			nlbCurrent.ID,
			nlbServiceCurrent.ID,
		); err != nil {
			return err
		}

		debugf("NLB service %s/%s deleted successfully", nlbCurrent.Name, nlbServiceCurrent.Name)
	}

	// Second loop: for each desired service, either update the existing one or create a new one.
	for _, nlbServiceUpdate := range nlbUpdate.Services {
		key := ServiceKey{Port: nlbServiceUpdate.Port, Protocol: nlbServiceUpdate.Protocol}

		debugf("Checking updated NLB service %s/%s - key %v",
			nlbUpdate.Name, nlbServiceUpdate.Name, key)

		// Check if there's an existing NLB service (same port/protocol)
		nlbServiceCurrent, ok := nlbServices[key]
		if !ok {
			// No existing one, so create brand new
			infof("creating new NLB service %s/%s", nlbCurrent.Name, nlbServiceUpdate.Name)

			_, err = l.p.client.AddServiceToLoadBalancer(ctx, nlbCurrent.ID, v3.AddServiceToLoadBalancerRequest{
				Name:        nlbServiceUpdate.Name,
				Description: nlbServiceUpdate.Description,
				Port:        nlbServiceUpdate.Port,
				TargetPort:  nlbServiceUpdate.TargetPort,
				Protocol:    v3.AddServiceToLoadBalancerRequestProtocol(nlbServiceUpdate.Protocol),
				Strategy:    v3.AddServiceToLoadBalancerRequestStrategy(nlbServiceUpdate.Strategy),
				Healthcheck: nlbServiceUpdate.Healthcheck,
				InstancePool: &v3.InstancePool{
					ID: nlbServiceUpdate.InstancePool.ID,
				},
			})
			if err != nil {
				return err
			}

			// Operation returns load balancer (not service) reference.
			// We now need to look for newly created service.
			nlb, err := l.p.client.GetLoadBalancer(ctx, nlbCurrent.ID)
			if err != nil {
				return err
			}

			svc := v3.LoadBalancerService{}
			for _, item := range nlb.Services {
				if item.Name == nlbServiceUpdate.Name {
					svc = item
				}
			}
			if svc.Name == "" {
				return fmt.Errorf(
					"failed to create NLB service %s/%s",
					nlbCurrent.Name,
					nlbServiceUpdate.Name,
				)
			}

			debugf("NLB service %s/%s created successfully (ID: %s)",
				nlbCurrent.Name,
				nlbServiceUpdate.Name,
				svc.ID,
			)
			continue
		}

		// We have an existing service with the same port/protocol, so let's see if the Instance Pool changed.

		nlbServiceUpdate.ID = nlbServiceCurrent.ID

		var currentPool v3.UUID
		if nlbServiceCurrent.InstancePool != nil {
			currentPool = nlbServiceCurrent.InstancePool.ID
		}

		var desiredPool v3.UUID
		if nlbServiceUpdate.InstancePool != nil {
			desiredPool = nlbServiceUpdate.InstancePool.ID
		}

		// If the InstancePoolID has changed, we must delete+recreate (API won't let us just "update" the pool with a new target).
		// https://openapi-v2.exoscale.com/operation/operation-update-load-balancer-service
		if currentPool != desiredPool {
			infof(
				"NLB service %s/%s target changed from %q to %q, must delete and recreate service",
				nlbCurrent.Name,
				nlbServiceCurrent.Name,
				currentPool,
				desiredPool,
			)

			// 1. Delete existing
			if _, err := l.p.client.DeleteLoadBalancerService(ctx, nlbCurrent.ID, nlbServiceCurrent.ID); err != nil {
				return fmt.Errorf("failed deleting NLB service: %w", err)
			}
			debugf("NLB service %s/%s deleted successfully", nlbCurrent.Name, nlbServiceCurrent.Name)

			// 2. Create fresh
			_, err = l.p.client.AddServiceToLoadBalancer(ctx, nlbCurrent.ID, v3.AddServiceToLoadBalancerRequest{
				Name:        nlbServiceUpdate.Name,
				Description: nlbServiceUpdate.Description,
				Port:        nlbServiceUpdate.Port,
				TargetPort:  nlbServiceUpdate.TargetPort,
				Protocol:    v3.AddServiceToLoadBalancerRequestProtocol(nlbServiceUpdate.Protocol),
				Strategy:    v3.AddServiceToLoadBalancerRequestStrategy(nlbServiceUpdate.Strategy),
				Healthcheck: nlbServiceUpdate.Healthcheck,
				InstancePool: &v3.InstancePool{
					ID: nlbServiceUpdate.InstancePool.ID,
				},
			})
			if err != nil {
				return err
			}

			// Operation returns load balancer (not service) reference.
			// We now need to look for newly created service.
			nlb, err := l.p.client.GetLoadBalancer(ctx, nlbCurrent.ID)
			if err != nil {
				return err
			}

			svc := v3.LoadBalancerService{}
			for _, item := range nlb.Services {
				if item.Name == nlbServiceUpdate.Name {
					svc = item
				}
			}
			if svc.Name == "" {
				return fmt.Errorf(
					"failed to create NLB service %s/%s",
					nlbCurrent.Name,
					nlbServiceUpdate.Name,
				)
			}

			debugf("NLB service %s/%s created successfully (ID: %s)",
				nlbCurrent.Name,
				nlbServiceUpdate.Name,
				svc.ID)

			continue
		}

		// Otherwise (pool is the same), just do a normal update if any other fields differ
		if isLoadBalancerServiceUpdated(nlbServiceCurrent, nlbServiceUpdate) {
			infof("updating NLB service %s/%s", nlbCurrent.Name, nlbServiceUpdate.Name)

			if _, err = l.p.client.UpdateLoadBalancerService(
				ctx,
				nlbUpdate.ID,
				nlbServiceUpdate.ID,
				v3.UpdateLoadBalancerServiceRequest{
					Name:        nlbServiceUpdate.Name,
					Description: nlbServiceUpdate.Description,
					Port:        nlbServiceUpdate.Port,
					TargetPort:  nlbServiceUpdate.TargetPort,
					Protocol:    v3.UpdateLoadBalancerServiceRequestProtocol(nlbServiceUpdate.Protocol),
					Strategy:    v3.UpdateLoadBalancerServiceRequestStrategy(nlbServiceUpdate.Strategy),
					Healthcheck: nlbServiceUpdate.Healthcheck,
				},
			); err != nil {
				return err
			}

			debugf("NLB service %s/%s updated successfully", nlbCurrent.Name, nlbServiceUpdate.Name)
		}

	}

	return nil
}

func (l *loadBalancer) fetchLoadBalancer(
	ctx context.Context,
	service *v1.Service,
) (*v3.LoadBalancer, error) {
	if lbID := getAnnotation(service, annotationLoadBalancerID, ""); lbID != "" {
		nlb, err := l.p.client.GetLoadBalancer(ctx, v3.UUID(lbID))
		if err != nil {
			if errors.Is(err, v3.ErrNotFound) {
				return nil, errLoadBalancerNotFound
			}

			return nil, err
		}

		return nlb, nil
	}

	return nil, errLoadBalancerNotFound
}

// fetchIPv6LoadBalancer returns the IPv6 NLB instance referenced in the Service annotations.
func (l *loadBalancer) fetchIPv6LoadBalancer(
	ctx context.Context,
	service *v1.Service,
) (*v3.LoadBalancer, error) {
	if lbID := getAnnotation(service, annotationLoadBalancerIPv6ID, ""); lbID != "" {
		nlb, err := l.p.client.GetLoadBalancer(ctx, v3.UUID(lbID))
		if err != nil {
			if errors.Is(err, v3.ErrNotFound) {
				return nil, errLoadBalancerNotFound
			}

			return nil, err
		}

		return nlb, nil
	}

	return nil, errLoadBalancerNotFound
}

// ensureIPv6LoadBalancer returns the IPv6 NLB instance of a dual-stack Service,
// resolving an external one by name or creating it if needed.
func (l *loadBalancer) ensureIPv6LoadBalancer(ctx context.Context, service *v1.Service) (*v3.LoadBalancer, error) {
	if l.isExternal(service) && getAnnotation(service, annotationLoadBalancerIPv6ID, "") == "" {
		lbName := getAnnotation(service, annotationLoadBalancerIPv6Name, "")

		nlbs, err := l.p.client.ListLoadBalancers(ctx)
		if err != nil {
			return nil, fmt.Errorf("error listing NLBs: %w", err)
		}

		nlb, err := nlbs.FindLoadBalancer(lbName)
		if err != nil {
			return nil, fmt.Errorf(
				"IPv6 NLB instance is marked external by name %q, but no matching NLB was found",
				lbName,
			)
		}

		if err := l.patchAnnotation(ctx, service, annotationLoadBalancerIPv6ID, nlb.ID.String()); err != nil {
			return nil, fmt.Errorf("error patching annotations: %w", err)
		}
		infof("found external IPv6 NLB %q by name %q and patched ID %s", nlb.Name, lbName, nlb.ID)
	}

	nlb, err := l.fetchIPv6LoadBalancer(ctx, service)
	if err == nil {
		if err := checkAddressFamily(nlb, v3.LoadBalancerAddressfamilyInet6); err != nil {
			return nil, err
		}

		return nlb, nil
	}

	if !errors.Is(err, errLoadBalancerNotFound) {
		return nil, err
	}

	if l.isExternal(service) {
		return nil, errors.New("IPv6 NLB instance marked as external in Service annotations, cannot create")
	}

	lbSpec, err := buildIPv6LoadBalancerFromAnnotations(service)
	if err != nil {
		return nil, err
	}

	infof("creating new IPv6 NLB %q", lbSpec.Name)

	op, err := l.p.client.CreateLoadBalancer(ctx, v3.CreateLoadBalancerRequest{
		Addressfamily: v3.CreateLoadBalancerRequestAddressfamilyInet6,
		Name:          lbSpec.Name,
		Description:   lbSpec.Description,
		Labels:        lbSpec.Labels,
	})
	if err != nil {
		return nil, err
	}

	nlb, err = l.p.client.GetLoadBalancer(ctx, op.Reference.ID)
	if err != nil {
		return nil, err
	}

	if err := l.patchAnnotation(ctx, service, annotationLoadBalancerIPv6ID, nlb.ID.String()); err != nil {
		return nil, fmt.Errorf("error patching annotations: %w", err)
	}

	debugf("IPv6 NLB %q created successfully (ID: %s)", nlb.Name, nlb.ID)

	return nlb, nil
}

// ensureIPv6LoadBalancerDeleted releases the IPv6 NLB instance of a Service that is no longer dual-stack,
// then removes its ID from the Service annotations.
func (l *loadBalancer) ensureIPv6LoadBalancerDeleted(ctx context.Context, service *v1.Service) error {
	nlb, err := l.fetchIPv6LoadBalancer(ctx, service)
	switch {
	case err == nil:
		infof("Service is no longer dualstack, releasing IPv6 NLB %q", nlb.Name)
		if err := l.deleteLoadBalancerServices(ctx, service, nlb); err != nil {
			return err
		}
	case !errors.Is(err, errLoadBalancerNotFound):
		return err
	}

	if err := l.removeAnnotation(ctx, service, annotationLoadBalancerIPv6ID); err != nil {
		return fmt.Errorf("error patching annotations: %w", err)
	}

	return nil
}

// checkInstancePoolDualStack returns an error if the Instance Pool targeted by the Service
// doesn't assign public IPv6 addresses to its members, since the IPv6 NLB would have no reachable backend.
// Members already known as cluster Nodes but without an IPv6 address yet are only logged.
func (l *loadBalancer) checkInstancePoolDualStack(ctx context.Context, service *v1.Service, nodes []*v1.Node) error {
	instancePoolID := v3.UUID(getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, ""))

	instancePool, err := l.p.client.GetInstancePool(ctx, instancePoolID)
	if err != nil {
		return fmt.Errorf("error retrieving Instance Pool %s: %w", instancePoolID, err)
	}

	isDual := instancePool.PublicIPAssignment == v3.PublicIPAssignmentDual ||
		(instancePool.Ipv6Enabled != nil && *instancePool.Ipv6Enabled)
	if !isDual {
		return fmt.Errorf(
			"instance pool %s is not dual-stack, which dualstack IP address type requires: "+
				"set its public IP assignment to dual (e.g. on the SKS nodepool)",
			instancePoolID,
		)
	}

	nodesByID := make(map[string]*v1.Node, len(nodes))
	for _, node := range nodes {
		nodesByID[strings.ToLower(node.Status.NodeInfo.SystemUUID)] = node
	}

	var missing []string
	for _, instance := range instancePool.Instances {
		node, ok := nodesByID[strings.ToLower(instance.ID.String())]
		if ok && !nodeHasIPv6Address(node) {
			missing = append(missing, node.Name)
		}
	}
	if len(missing) > 0 {
		infof("warning: Nodes %s have no IPv6 address yet, the IPv6 NLB won't route traffic to them",
			strings.Join(missing, ", "))
	}

	return nil
}

func nodeHasIPv6Address(node *v1.Node) bool {
	for _, addr := range node.Status.Addresses {
		if ip := net.ParseIP(addr.Address); ip != nil && ip.To4() == nil {
			return true
		}
	}

	return false
}

func (l *loadBalancer) removeAnnotation(ctx context.Context, service *v1.Service, k string) error {
	if _, ok := service.Annotations[k]; !ok {
		return nil
	}

	patcher := newServicePatcher(ctx, l.p.kclient, service)

	delete(service.Annotations, k)

	return patcher.Patch()
}

func (l *loadBalancer) patchAnnotation(ctx context.Context, service *v1.Service, k, v string) error {
	patcher := newServicePatcher(ctx, l.p.kclient, service)

	if service.Annotations == nil {
		service.Annotations = map[string]string{}
	}

	if cur, ok := service.Annotations[k]; ok && cur == v {
		return nil
	}

	service.Annotations[k] = v

	return patcher.Patch()
}

func (c *refreshableExoscaleClient) CreateLoadBalancer(
	ctx context.Context,
	req v3.CreateLoadBalancerRequest,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.CreateLoadBalancer(
		ctx,
		req,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *refreshableExoscaleClient) AddServiceToLoadBalancer(
	ctx context.Context,
	id v3.UUID,
	req v3.AddServiceToLoadBalancerRequest,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.AddServiceToLoadBalancer(
		ctx,
		id,
		req,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *refreshableExoscaleClient) DeleteLoadBalancer(
	ctx context.Context,
	id v3.UUID,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.DeleteLoadBalancer(
		ctx,
		id,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *refreshableExoscaleClient) DeleteLoadBalancerService(
	ctx context.Context,
	id v3.UUID,
	serviceID v3.UUID,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.DeleteLoadBalancerService(
		ctx,
		id,
		serviceID,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *refreshableExoscaleClient) GetLoadBalancer(
	ctx context.Context,
	id v3.UUID,
) (*v3.LoadBalancer, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetLoadBalancer(
		ctx,
		id,
	)
}

func (c *refreshableExoscaleClient) GetInstancePool(
	ctx context.Context,
	id v3.UUID,
) (*v3.InstancePool, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetInstancePool(
		ctx,
		id,
	)
}

func (c *refreshableExoscaleClient) ListLoadBalancers(
	ctx context.Context,
) (*v3.ListLoadBalancersResponse, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.ListLoadBalancers(
		ctx,
	)
}

func (c *refreshableExoscaleClient) UpdateLoadBalancer(
	ctx context.Context,
	id v3.UUID,
	req v3.UpdateLoadBalancerRequest,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.UpdateLoadBalancer(
		ctx,
		id,
		req,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func (c *refreshableExoscaleClient) UpdateLoadBalancerService(
	ctx context.Context,
	id v3.UUID,
	serviceID v3.UUID,
	req v3.UpdateLoadBalancerServiceRequest,
) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	op, err := c.exo.UpdateLoadBalancerService(
		ctx,
		id,
		serviceID,
		req,
	)
	if err != nil {
		return nil, err
	}

	return c.exo.Wait(ctx, op, v3.OperationStateSuccess)
}

func getAnnotation(service *v1.Service, annotation, defaultValue string) string {
	v, ok := service.Annotations[annotation]
	if ok {
		return v
	}

	if defaultValue != "" {
		return defaultValue
	}

	return ""
}

func buildLoadBalancerFromAnnotations(service *v1.Service) (*v3.LoadBalancer, error) {
	lb := v3.LoadBalancer{
		ID:          v3.UUID(getAnnotation(service, annotationLoadBalancerID, "")),
		Name:        getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID)),
		Description: getAnnotation(service, annotationLoadBalancerDescription, ""),
		Services:    make([]v3.LoadBalancerService, 0),
	}

	hcInterval, err := time.ParseDuration(getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckInterval,
		defaultNLBServiceHealthcheckInterval,
	))
	if err != nil {
		return nil, err
	}

	hcTimeout, err := time.ParseDuration(getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckTimeout,
		defaultNLBServiceHealthCheckTimeout,
	))
	if err != nil {
		return nil, err
	}

	hcRetriesI, err := strconv.Atoi(getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckRetries,
		fmt.Sprint(defaultNLBServiceHealthcheckRetries),
	))
	if err != nil {
		return nil, err
	}
	hcRetries := int64(hcRetriesI)

	for _, servicePort := range service.Spec.Ports {
		var hcPort uint16

		// If the user specifies a healthcheck port in the Service manifest annotations, we use that
		// that is important for UDP services, as there the user must specify a TCP nodeport for healthchecks
		hcPortAnnotation := getAnnotation(service, annotationLoadBalancerServiceHealthCheckPort, "")
		if hcPortAnnotation != "" {
			hcPortInt, err := strconv.Atoi(hcPortAnnotation)
			if err != nil {
				return nil, fmt.Errorf("invalid healthcheck port annotation: %s", err)
			}
			hcPort = uint16(hcPortInt)
		} else {
			// If the Service is configured with externalTrafficPolicy=Local, we use the value of the
			// healthCheckNodePort property as NLB service healthcheck port as explained in this article:
			// https://kubernetes.io/docs/tutorials/services/source-ip/#source-ip-for-services-with-type-loadbalancer
			// TL;DR: this configures the NLB service to ensure only Instance Pool members actually running
			// an endpoint for the corresponding K8s Service will receive ingress traffic from the NLB, thus
			// preserving the source IP address information.
			hcPort = uint16(servicePort.NodePort)
			if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal &&
				service.Spec.HealthCheckNodePort > 0 {
				debugf("Service is configured with externalPolicy:Local, "+
					"using the Service spec.healthCheckNodePort value (%d) instead "+
					"of NodePort (%d) for NLB service healthcheck port",
					service.Spec.HealthCheckNodePort,
					servicePort.NodePort)
				hcPort = uint16(service.Spec.HealthCheckNodePort)
			}
		}

		// We support TCP/UDP but not SCTP
		if servicePort.Protocol != v1.ProtocolTCP && servicePort.Protocol != v1.ProtocolUDP {
			return nil, errors.New("only TCP and UDP are supported as service port protocols")
		}

		var (
			// Name must be unique for updateLoadBalancer to work correctly
			svcName       = fmt.Sprintf("%s-%d", service.UID, servicePort.Port)
			svcProtocol   = v3.LoadBalancerServiceProtocol(strings.ToLower(string(servicePort.Protocol)))
			svcPort       = int64(servicePort.Port)
			svcTargetPort = int64(servicePort.NodePort)
		)

		if servicePort.Protocol != v1.ProtocolTCP { // Add protocol to service name if not TCP
			svcName += "-" + strings.ToLower(string(servicePort.Protocol))
		}

		svc := v3.LoadBalancerService{
			Healthcheck: &v3.LoadBalancerServiceHealthcheck{
				Mode: v3.LoadBalancerServiceHealthcheckMode(getAnnotation(
					service,
					annotationLoadBalancerServiceHealthCheckMode,
					string(defaultNLBServiceHealthcheckMode),
				)),
				Port:     int64(hcPort),
				URI:      getAnnotation(service, annotationLoadBalancerServiceHealthCheckURI, ""),
				Interval: int64(hcInterval.Seconds()), // TODO refacto here
				Timeout:  int64(hcTimeout.Seconds()),  // TODO refacto here
				Retries:  hcRetries,
			},
			InstancePool: &v3.InstancePool{
				ID: v3.UUID(getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, "")),
			},
			Name:     svcName,
			Port:     svcPort,
			Protocol: svcProtocol,
			Strategy: v3.LoadBalancerServiceStrategy(getAnnotation(
				service,
				annotationLoadBalancerServiceStrategy,
				string(defaultNLBServiceStrategy),
			)),
			TargetPort: svcTargetPort,
		}

		// If there is only one service port defined, allow additional NLB service properties
		// to be set via annotations, as setting those from annotations would not make sense
		// if multiple NLB services co-exist on the same NLB instance (e.g. name, description).
		if len(service.Spec.Ports) == 1 {
			svc.Name = getAnnotation(service, annotationLoadBalancerServiceName, svc.Name)
			svc.Description = getAnnotation(service, annotationLoadBalancerServiceDescription, "")
		}

		lb.Services = append(lb.Services, svc)
	}

	return &lb, nil
}

// getIPAddressType returns the NLB IP address type requested in the Service annotations.
func getIPAddressType(service *v1.Service) (string, error) {
	switch v := strings.ToLower(getAnnotation(service, annotationLoadBalancerIPAddressType, ipAddressTypeIPv4)); v {
	case ipAddressTypeIPv4, ipAddressTypeDualStack:
		return v, nil
	case ipAddressTypeIPv6:
		return "", fmt.Errorf("%s %q is not supported yet, use %q", annotationLoadBalancerIPAddressType, v, ipAddressTypeDualStack)
	default:
		return "", fmt.Errorf("invalid %s %q: expected %q or %q",
			annotationLoadBalancerIPAddressType, v, ipAddressTypeIPv4, ipAddressTypeDualStack)
	}
}

// parseIPv6TargetPorts returns the IPv6 target port of each Service port, keyed by Service port.
// The annotation value is a list of "<service port>:<target port>" pairs, e.g. "80:8080,443:8443",
// or a single target port if the Service has only one port.
// Nothing serves the NodePorts over IPv6, so every Service port must be mapped.
func parseIPv6TargetPorts(service *v1.Service) (map[int32]int64, error) {
	v := strings.TrimSpace(getAnnotation(service, annotationLoadBalancerIPv6TargetPorts, ""))
	if v == "" {
		return nil, fmt.Errorf("annotation %s is required with %s %q",
			annotationLoadBalancerIPv6TargetPorts, annotationLoadBalancerIPAddressType, ipAddressTypeDualStack)
	}

	parsePort := func(s string) (int64, error) {
		port, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
		if err != nil || port < 1 || port > 65535 {
			return 0, fmt.Errorf("invalid port %q in annotation %s", s, annotationLoadBalancerIPv6TargetPorts)
		}
		return port, nil
	}

	servicePorts := make(map[int32]bool, len(service.Spec.Ports))
	for _, servicePort := range service.Spec.Ports {
		servicePorts[servicePort.Port] = true
	}

	targetPorts := make(map[int32]int64, len(service.Spec.Ports))

	if !strings.Contains(v, ":") {
		if len(servicePorts) != 1 {
			return nil, fmt.Errorf(
				"annotation %s must use the <service port>:<target port> format when the Service has several ports",
				annotationLoadBalancerIPv6TargetPorts,
			)
		}

		targetPort, err := parsePort(v)
		if err != nil {
			return nil, err
		}

		targetPorts[service.Spec.Ports[0].Port] = targetPort

		return targetPorts, nil
	}

	for _, pair := range strings.Split(v, ",") {
		port, target, ok := strings.Cut(pair, ":")
		if !ok {
			return nil, fmt.Errorf("invalid entry %q in annotation %s, expected <service port>:<target port>",
				pair, annotationLoadBalancerIPv6TargetPorts)
		}

		servicePort, err := parsePort(port)
		if err != nil {
			return nil, err
		}

		targetPort, err := parsePort(target)
		if err != nil {
			return nil, err
		}

		if !servicePorts[int32(servicePort)] {
			return nil, fmt.Errorf("annotation %s references port %d, which the Service doesn't expose",
				annotationLoadBalancerIPv6TargetPorts, servicePort)
		}

		if _, ok := targetPorts[int32(servicePort)]; ok {
			return nil, fmt.Errorf("annotation %s maps port %d more than once",
				annotationLoadBalancerIPv6TargetPorts, servicePort)
		}

		targetPorts[int32(servicePort)] = targetPort
	}

	for port := range servicePorts {
		if _, ok := targetPorts[port]; !ok {
			return nil, fmt.Errorf("annotation %s has no target port for Service port %d",
				annotationLoadBalancerIPv6TargetPorts, port)
		}
	}

	return targetPorts, nil
}

// buildIPv6LoadBalancerFromAnnotations returns the IPv6 NLB instance spec of a dual-stack Service.
// It only differs from the IPv4 one by its ID and name, and by its services targeting the IPv6
// target ports (health checked on these ports too, unless overridden) instead of the NodePorts.
func buildIPv6LoadBalancerFromAnnotations(service *v1.Service) (*v3.LoadBalancer, error) {
	lb, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return nil, err
	}

	targetPorts, err := parseIPv6TargetPorts(service)
	if err != nil {
		return nil, err
	}

	var hcPort int64
	if v := getAnnotation(service, annotationLoadBalancerIPv6HealthCheckPort, ""); v != "" {
		port, err := strconv.ParseInt(v, 10, 32)
		if err != nil || port < 1 || port > 65535 {
			return nil, fmt.Errorf("invalid IPv6 healthcheck port annotation: %q", v)
		}
		hcPort = port
	}

	lb.ID = v3.UUID(getAnnotation(service, annotationLoadBalancerIPv6ID, ""))
	lb.Name = getAnnotation(service, annotationLoadBalancerIPv6Name, lb.Name+"-ipv6")

	// NLB services are built in the Service ports order.
	for i, servicePort := range service.Spec.Ports {
		lb.Services[i].TargetPort = targetPorts[servicePort.Port]
		lb.Services[i].Healthcheck.Port = lb.Services[i].TargetPort
		if hcPort != 0 {
			lb.Services[i].Healthcheck.Port = hcPort
		}
	}

	return lb, nil
}

// checkAddressFamily returns an error if the NLB instance doesn't have the expected address family.
// NLB instances created before the address family was introduced are IPv4.
func checkAddressFamily(nlb *v3.LoadBalancer, want v3.LoadBalancerAddressfamily) error {
	family := nlb.Addressfamily
	if family == "" {
		family = v3.LoadBalancerAddressfamilyInet4
	}

	if family != want {
		return fmt.Errorf("NLB %q (%s) has address family %q, expected %q: "+
			"check the %s/%s annotations, or whether this NLB is shared with another Service",
			nlb.Name, nlb.ID, family, want, annotationLoadBalancerID, annotationLoadBalancerIPv6ID)
	}

	return nil
}

// loadBalancerStatus returns the Service status of the NLB instances, IPv4 first.
func loadBalancerStatus(nlb, nlbIPv6 *v3.LoadBalancer) *v1.LoadBalancerStatus {
	status := &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: nlb.IP.String()}}}

	if nlbIPv6 != nil {
		status.Ingress = append(status.Ingress, v1.LoadBalancerIngress{IP: nlbIPv6.IP.String()})
	}

	return status
}

func isLoadBalancerUpdated(current, update *v3.LoadBalancer) bool {
	if current.Name != update.Name {
		return true
	}

	if current.Description != update.Description {
		return true
	}

	return false
}

func isLoadBalancerServiceUpdated(current, update v3.LoadBalancerService) bool {
	return !cmp.Equal(current, update, cmpopts.IgnoreFields(current, "State", "HealthcheckStatus"))
}
