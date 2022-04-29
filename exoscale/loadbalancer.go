package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	egoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
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
	annotationLoadBalancerServiceHealthCheckMode     = annotationPrefix + "service-healthcheck-mode"
	annotationLoadBalancerServiceHealthCheckURI      = annotationPrefix + "service-healthcheck-uri"
	annotationLoadBalancerServiceHealthCheckInterval = annotationPrefix + "service-healthcheck-interval"
	annotationLoadBalancerServiceHealthCheckTimeout  = annotationPrefix + "service-healthcheck-timeout"
	annotationLoadBalancerServiceHealthCheckRetries  = annotationPrefix + "service-healthcheck-retries"
)

var (
	defaultNLBServiceHealthCheckTimeout        = "5s"
	defaultNLBServiceHealthcheckInterval       = "10s"
	defaultNLBServiceHealthcheckMode           = "tcp"
	defaultNLBServiceHealthcheckRetries  int64 = 1
	defaultNLBServiceStrategy                  = "round-robin"
)

var errLoadBalancerNotFound = errors.New("load balancer not found")

type loadBalancer struct {
	p   *cloudProvider
	cfg *loadBalancerConfig
}

// isExternal returns true if the NLB instance is marked as "external" in the
// Kubernetes Service manifest annotations (i.e. not managed by the CCM).
func (l loadBalancer) isExternal(service *v1.Service) bool {
	return strings.ToLower(*getAnnotation(service, annotationLoadBalancerExternal, "false")) == "true"
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

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: nlb.IPAddress.String()}}}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancer) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return *getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID))
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
	// Inferring the Instance Pool ID from the cluster Nodes that run the Service in case no Instance Pool ID
	// has been specified in the annotations.
	//
	// IMPORTANT: this use case is not compatible with Services referencing Pods using Node Selectors
	// (see https://github.com/kubernetes/kubernetes/issues/45234 for an explanation of the problem).
	// The list of Nodes passed as argument to this method contains *ALL* the Nodes in the cluster, not only the
	// ones that actually host the Pods targeted by the Service.
	if getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, "") == nil {
		debugf("no NLB service Instance Pool ID specified in Service annotations, inferring from cluster Nodes")

		instancePoolID := ""
		for _, node := range nodes {
			instance, err := l.p.client.GetInstance(ctx, l.p.zone, node.Status.NodeInfo.SystemUUID)
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

		err := l.patchAnnotation(ctx, service, annotationLoadBalancerServiceInstancePoolID, instancePoolID)
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

			infof("creating new NLB %q", *lbSpec.Name)

			nlb, err = l.p.client.CreateNetworkLoadBalancer(ctx, l.p.zone, lbSpec)
			if err != nil {
				return nil, err
			}

			if err := l.patchAnnotation(ctx, service, annotationLoadBalancerID, *nlb.ID); err != nil {
				return nil, fmt.Errorf("error patching annotations: %s", err)
			}

			debugf("NLB %q created successfully (ID: %s)", *nlb.Name, *nlb.ID)
		} else {
			return nil, err
		}
	}

	if err = l.updateLoadBalancer(ctx, service); err != nil {
		return nil, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: nlb.IPAddress.String()}}}, nil
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
			if int32(*nlbService.Port) == servicePort.Port {
				infof("deleting NLB service %s/%s", *nlb.Name, *nlbService.Name)
				if err = l.p.client.DeleteNetworkLoadBalancerService(ctx, l.p.zone, nlb, nlbService); err != nil {
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

		infof("deleting NLB %q", *nlb.Name)

		return l.p.client.DeleteNetworkLoadBalancer(ctx, l.p.zone, nlb)
	}

	return nil
}

// updateLoadBalancer updates the matching Exoscale NLB instance according to the *v1.Service spec provided.
func (l *loadBalancer) updateLoadBalancer(ctx context.Context, service *v1.Service) error {
	nlbUpdate, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return err
	}

	nlbCurrent, err := l.p.client.GetNetworkLoadBalancer(ctx, l.p.zone, *nlbUpdate.ID)
	if err != nil {
		return err
	}

	if !l.isExternal(service) && isLoadBalancerUpdated(nlbCurrent, nlbUpdate) {
		infof("updating NLB %q", *nlbCurrent.Name)

		if err = l.p.client.UpdateNetworkLoadBalancer(ctx, l.p.zone, nlbUpdate); err != nil {
			return err
		}

		debugf("NLB %q updated successfully", *nlbCurrent.Name)
	}

	// Delete the NLB services which port is not present in the updated version.
	nlbServices := make(map[uint16]*egoscale.NetworkLoadBalancerService)
next:
	for _, nlbServiceCurrent := range nlbCurrent.Services {
		for _, nlbServiceUpdate := range nlbUpdate.Services {
			// If a service exposing the same port already exists,
			// flag it for update and save its ID for later reference.
			if *nlbServiceUpdate.Port == *nlbServiceCurrent.Port {
				debugf("Service port %d already in use by NLB service %s/%s, marking for update",
					*nlbServiceCurrent.Port,
					*nlbCurrent.Name,
					*nlbServiceCurrent.Name)
				nlbServices[*nlbServiceCurrent.Port] = nlbServiceCurrent
				continue next
			}
		}

		if l.isExternal(service) {
			debugf("NLB service %s/%s doesn't match any service port, but this Service is "+
				"using an external NLB. Avoiding deletion since it may belong to another Service",
				*nlbCurrent.Name,
				*nlbServiceCurrent.Name)
			continue next
		}

		infof("NLB service %s/%s doesn't match any service port, deleting",
			*nlbCurrent.Name,
			*nlbServiceCurrent.Name)

		if err := l.p.client.DeleteNetworkLoadBalancerService(
			ctx,
			l.p.zone,
			nlbCurrent,
			nlbServiceCurrent,
		); err != nil {
			return err
		}

		debugf("NLB service %s/%s deleted successfully", *nlbCurrent.Name, *nlbServiceCurrent.Name)
	}

	// Update existing services and add new ones.
	for _, nlbServiceUpdate := range nlbUpdate.Services {
		if nlbServiceCurrent, ok := nlbServices[*nlbServiceUpdate.Port]; ok {
			nlbServiceUpdate.ID = nlbServiceCurrent.ID
			if isLoadBalancerServiceUpdated(nlbServiceCurrent, nlbServiceUpdate) {
				infof("updating NLB service %s/%s", *nlbCurrent.Name, *nlbServiceUpdate.Name)

				if err = l.p.client.UpdateNetworkLoadBalancerService(
					ctx,
					l.p.zone,
					nlbUpdate,
					nlbServiceUpdate,
				); err != nil {
					return err
				}

				debugf("NLB service %s/%s updated successfully", *nlbCurrent.Name, *nlbServiceUpdate.Name)
			}
		} else {
			infof("creating new NLB service %s/%s", *nlbCurrent.Name, *nlbServiceUpdate.Name)

			svc, err := l.p.client.CreateNetworkLoadBalancerService(ctx, l.p.zone, nlbCurrent, nlbServiceUpdate)
			if err != nil {
				return err
			}

			debugf("NLB service %s/%s created successfully (ID: %s)",
				*nlbCurrent.Name,
				*nlbServiceUpdate.Name,
				*svc.ID)
		}
	}

	return nil
}

func (l *loadBalancer) fetchLoadBalancer(
	ctx context.Context,
	service *v1.Service,
) (*egoscale.NetworkLoadBalancer, error) {
	if lbID := getAnnotation(service, annotationLoadBalancerID, ""); lbID != nil {
		nlb, err := l.p.client.GetNetworkLoadBalancer(ctx, l.p.zone, *lbID)
		switch err {
		case nil:
			return nlb, nil

		case exoapi.ErrNotFound:
			return nil, errLoadBalancerNotFound

		default:
			return nil, err
		}
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

func (c *refreshableExoscaleClient) CreateNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) (*egoscale.NetworkLoadBalancer, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.CreateNetworkLoadBalancer(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
	)
}

func (c *refreshableExoscaleClient) CreateNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService,
) (*egoscale.NetworkLoadBalancerService, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.CreateNetworkLoadBalancerService(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
		svc,
	)
}

func (c *refreshableExoscaleClient) DeleteNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) error {
	c.RLock()
	defer c.RUnlock()

	return c.exo.DeleteNetworkLoadBalancer(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
	)
}

func (c *refreshableExoscaleClient) DeleteNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService) error {
	c.RLock()
	defer c.RUnlock()

	return c.exo.DeleteNetworkLoadBalancerService(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
		svc,
	)
}

func (c *refreshableExoscaleClient) GetNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	id string,
) (*egoscale.NetworkLoadBalancer, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetNetworkLoadBalancer(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		id,
	)
}

func (c *refreshableExoscaleClient) UpdateNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) error {
	c.RLock()
	defer c.RUnlock()

	return c.exo.UpdateNetworkLoadBalancer(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
	)
}

func (c *refreshableExoscaleClient) UpdateNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService,
) error {
	c.RLock()
	defer c.RUnlock()

	return c.exo.UpdateNetworkLoadBalancerService(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		nlb,
		svc,
	)
}

func getAnnotation(service *v1.Service, annotation, defaultValue string) *string {
	v, ok := service.Annotations[annotation]
	if ok {
		return &v
	}

	if defaultValue != "" {
		return &defaultValue
	}

	return nil
}

func buildLoadBalancerFromAnnotations(service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	lb := egoscale.NetworkLoadBalancer{
		ID:          getAnnotation(service, annotationLoadBalancerID, ""),
		Name:        getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID)),
		Description: getAnnotation(service, annotationLoadBalancerDescription, ""),
		Services:    make([]*egoscale.NetworkLoadBalancerService, 0),
	}

	hcInterval, err := time.ParseDuration(*getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckInterval,
		defaultNLBServiceHealthcheckInterval,
	))
	if err != nil {
		return nil, err
	}

	hcTimeout, err := time.ParseDuration(*getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckTimeout,
		defaultNLBServiceHealthCheckTimeout,
	))
	if err != nil {
		return nil, err
	}

	hcRetriesI, err := strconv.Atoi(*getAnnotation(
		service,
		annotationLoadBalancerServiceHealthCheckRetries,
		fmt.Sprint(defaultNLBServiceHealthcheckRetries),
	))
	if err != nil {
		return nil, err
	}
	hcRetries := int64(hcRetriesI)

	for _, servicePort := range service.Spec.Ports {
		// If the Service is configured with externalTrafficPolicy=Local, we use the value of the
		// healthCheckNodePort property as NLB service healthcheck port as explained in this article:
		// https://kubernetes.io/docs/tutorials/services/source-ip/#source-ip-for-services-with-type-loadbalancer
		// TL;DR: this configures the NLB service to ensure only Instance Pool members actually running
		// an endpoint for the corresponding K8s Service will receive ingress traffic from the NLB, thus
		// preserving the source IP address information.
		hcPort := uint16(servicePort.NodePort)
		if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal &&
			service.Spec.HealthCheckNodePort > 0 {
			debugf("Service is configured with externalPolicy:Local, "+
				"using the Service spec.healthCheckNodePort value (%d) instead "+
				"of NodePort (%d) for NLB service healthcheck port",
				service.Spec.HealthCheckNodePort,
				servicePort.NodePort)
			hcPort = uint16(service.Spec.HealthCheckNodePort)
		}

		// Exoscale NLB services can forward both TCP and UDP protocol, however the only supported
		// healthcheck protocol is TCP (plain TCP or HTTP).
		// Due to a technical limitation in Kubernetes preventing declaration of mixed protocols in a
		// service of type LoadBalancer (https://github.com/kubernetes/kubernetes/issues/23880) we only
		// allow TCP for service ports.
		if servicePort.Protocol != v1.ProtocolTCP {
			return nil, errors.New("only TCP is supported as service port protocol")
		}

		var (
			svcName       = fmt.Sprintf("%s-%d", service.UID, servicePort.Port)
			svcProtocol   = strings.ToLower(string(servicePort.Protocol))
			svcPort       = uint16(servicePort.Port)
			svcTargetPort = uint16(servicePort.NodePort)
		)

		svc := egoscale.NetworkLoadBalancerService{
			Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
				Mode: getAnnotation(
					service,
					annotationLoadBalancerServiceHealthCheckMode,
					defaultNLBServiceHealthcheckMode,
				),
				Port:     &hcPort,
				URI:      getAnnotation(service, annotationLoadBalancerServiceHealthCheckURI, ""),
				Interval: &hcInterval,
				Timeout:  &hcTimeout,
				Retries:  &hcRetries,
			},
			InstancePoolID: getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, ""),
			Name:           &svcName,
			Port:           &svcPort,
			Protocol:       &svcProtocol,
			Strategy: getAnnotation(
				service,
				annotationLoadBalancerServiceStrategy,
				defaultNLBServiceStrategy,
			),
			TargetPort: &svcTargetPort,
		}

		// If there is only one service port defined, allow additional NLB service properties
		// to be set via annotations, as setting those from annotations would not make sense
		// if multiple NLB services co-exist on the same NLB instance (e.g. name, description).
		if len(service.Spec.Ports) == 1 {
			svc.Name = getAnnotation(service, annotationLoadBalancerServiceName, *svc.Name)
			svc.Description = getAnnotation(service, annotationLoadBalancerServiceDescription, "")
		}

		lb.Services = append(lb.Services, &svc)
	}

	return &lb, nil
}

func isLoadBalancerUpdated(current, update *egoscale.NetworkLoadBalancer) bool {
	if defaultString(current.Name, "") != defaultString(update.Name, "") {
		return true
	}

	if defaultString(current.Description, "") != defaultString(update.Description, "") {
		return true
	}

	return false
}

func isLoadBalancerServiceUpdated(current, update *egoscale.NetworkLoadBalancerService) bool {
	return !cmp.Equal(current, update, cmpopts.IgnoreFields(*current, "State", "HealthcheckStatus"))
}
