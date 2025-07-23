package exoscale

import (
	"context"
	"errors"
	"fmt"
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

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: nlb.IP.String()}}}, true, nil
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
				if strings.EqualFold(pool.Name, sksNodePoolName) {
					instancePoolID = pool.ID
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

	if err = l.updateLoadBalancer(ctx, service); err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: nlb.IP.String()}}}, nil
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
	nlb, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if errors.Is(err, errLoadBalancerNotFound) {
			return nil
		}

		return err
	}

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

// updateLoadBalancer updates the matching Exoscale NLB instance according to the *v1.Service spec provided.
func (l *loadBalancer) updateLoadBalancer(ctx context.Context, service *v1.Service) error {
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

			svc, err := l.p.client.AddServiceToLoadBalancer(ctx, nlbCurrent.ID, v3.AddServiceToLoadBalancerRequest{
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
			svc, err := l.p.client.AddServiceToLoadBalancer(ctx, nlbCurrent.ID, v3.AddServiceToLoadBalancerRequest{
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
