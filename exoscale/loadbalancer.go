package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	annotationLoadBalancerID = "service.beta.kubernetes.io/exoscale-loadbalancer-id"

	annotationLoadBalancerName = "service.beta.kubernetes.io/exoscale-loadbalancer-name"

	annotationLoadBalancerDescription = "service.beta.kubernetes.io/exoscale-loadbalancer-description"

	// the possible values are "bg-sof-1", "ch-dk-2", "ch-gva-2", "de-fra-1", "de-muc-1"
	annotationLoadBalancerZone = "service.beta.kubernetes.io/exoscale-loadbalancer-zone"

	annotationLoadBalancerServiceID = "service.beta.kubernetes.io/exoscale-loadbalancer-service-id"

	annotationLoadBalancerServiceName = "service.beta.kubernetes.io/exoscale-loadbalancer-service-name"

	annotationLoadBalancerServiceDescription = "service.beta.kubernetes.io/exoscale-loadbalancer-service-description"

	annotationLoadBalancerServiceInstancePoolID = "service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id"

	// the possible values are "round-robin" or "source-hash"
	annotationLoadBalancerServiceStrategy = "service.beta.kubernetes.io/exoscale-loadbalancer-service-strategy"

	// the possible values are "tcp" or "http"
	annotationLoadBalancerServiceProtocol = "service.beta.kubernetes.io/exoscale-loadbalancer-service-protocol"

	// the default value is "tcp" and the value can be "http"
	annotationLoadBalancerServiceHealthCheckMode = "service.beta.kubernetes.ioexoscale-loadbalancer-service-healthcheck-mode"

	annotationLoadBalancerServiceHealthCheckInterval = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval"

	annotationLoadBalancerServiceHealthCheckTimeout = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout"

	annotationLoadBalancerServiceHealthCheckRetries = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries"

	// the default value is "/"
	annotationLoadBalancerServiceHealthCheckHTTPURI = "service.beta.kubernetes.io/exoscale-loadbalancer-service-http-healthcheck-uri"
)

var (
	errLoadBalancerNotFound        = errors.New("load balancer not found")
	errLoadBalancerServiceNotFound = errors.New("load balancer service not found")
)

type loadBalancer struct {
	p *cloudProvider
}

func newLoadBalancer(provider *cloudProvider) cloudprovider.LoadBalancer {
	return &loadBalancer{
		p: provider,
	}
}

// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	lb, _, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IPAddress.String(),
			},
		},
	}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancer) GetLoadBalancerName(_ context.Context, clusterName string, service *v1.Service) string {
	return getAnnotation(service, annotationLoadBalancerName, "nlb-"+string(service.UID))
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, _ []*v1.Node) (*v1.LoadBalancerStatus, error) {
	var lb *egoscale.NetworkLoadBalancer

	kubelb, err := buildLoadBalancerWithAnnotations(service)
	if err != nil {
		return nil, err
	}

	_, zone, err := l.fetchLoadBalancer(ctx, service)
	switch err {
	case nil:
		lb, err = l.p.client.UpdateNetworkLoadBalancer(ctx, zone, kubelb)
		if err != nil {
			return nil, err
		}
	case errLoadBalancerNotFound:
		lb, err = l.p.client.CreateNetworkLoadBalancer(ctx, zone, kubelb)
		if err != nil {
			return nil, err
		}

		if err := l.annotationLoadbalancerPatch(service, lb); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	_, err = l.fetchLoadBalancerService(lb, service)
	switch err {
	case nil:
		if err := lb.UpdateService(ctx, kubelb.Services[0]); err != nil {
			return nil, err
		}
	case errLoadBalancerServiceNotFound:
		lbService, err := lb.AddService(ctx, kubelb.Services[0])
		if err != nil {
			return nil, err
		}

		if err := l.annotationLoadbalancerServicePatch(service, lbService); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}

	return &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				IP: lb.IPAddress.String(),
			},
		},
	}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, _ []*v1.Node) error {
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return err
	}

	kubelb, err := buildLoadBalancerWithAnnotations(service)
	if err != nil {
		return err
	}

	lb, err := l.p.client.UpdateNetworkLoadBalancer(ctx, zone, kubelb)
	if err != nil {
		return err
	}
	if err := lb.UpdateService(ctx, kubelb.Services[0]); err != nil {
		return err
	}

	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	lb, zone, err := l.fetchLoadBalancer(ctx, service)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil
		}

		return err
	}

	if len(lb.Services) == 1 {
		return l.p.client.DeleteNetworkLoadBalancer(ctx, zone, lb.ID)
	}

	lbService, err := l.fetchLoadBalancerService(lb, service)
	if err != nil {
		return err
	}

	return lb.DeleteService(ctx, lbService)
}

func (l *loadBalancer) annotationLoadbalancerPatch(service *v1.Service, lb *egoscale.NetworkLoadBalancer) error {
	patcher := newServicePatcher(l.p.kclient, service)

	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationLoadBalancerID] = lb.ID

	return patcher.Patch()
}

func (l *loadBalancer) annotationLoadbalancerServicePatch(service *v1.Service, lbService *egoscale.NetworkLoadBalancerService) error {
	patcher := newServicePatcher(l.p.kclient, service)

	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationLoadBalancerServiceID] = lbService.ID

	return patcher.Patch()
}

func (l *loadBalancer) fetchLoadBalancer(ctx context.Context, service *v1.Service) (*egoscale.NetworkLoadBalancer, string, error) {
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return nil, "", err
	}

	if lbID := getAnnotation(service, annotationLoadBalancerID, ""); lbID != "" {
		lb, err := l.p.client.GetNetworkLoadBalancer(ctx, zone, lbID)
		if err != nil {
			return nil, "", err
		}

		return lb, zone, nil
	}

	resp, err := l.p.client.ListNetworkLoadBalancers(ctx, zone)
	if err != nil {
		return nil, "", err
	}

	var loadbalancers []*egoscale.NetworkLoadBalancer
	for _, lb := range resp {
		if lb.Name == getAnnotation(service, annotationLoadBalancerName, "nlb-"+string(service.UID)) {
			loadbalancers = append(loadbalancers, lb)
		}
	}

	switch count := len(loadbalancers); {
	case count == 0:
		return nil, zone, errLoadBalancerNotFound
	case count > 1:
		return nil, "", errors.New("more than one element found")
	}

	if err := l.annotationLoadbalancerPatch(service, loadbalancers[0]); err != nil {
		return nil, "", err
	}

	return loadbalancers[0], zone, nil
}

func (l *loadBalancer) fetchLoadBalancerService(lb *egoscale.NetworkLoadBalancer, service *v1.Service) (*egoscale.NetworkLoadBalancerService, error) {
	var lbService []*egoscale.NetworkLoadBalancerService

	for _, svc := range lb.Services {
		if svc.ID == getAnnotation(service, annotationLoadBalancerServiceID, "") {
			return svc, nil
		}
		if svc.Name == getAnnotation(service, annotationLoadBalancerServiceName, "nlb-service-"+string(service.UID)) {
			lbService = append(lbService, svc)
		}
	}

	switch count := len(lbService); {
	case count == 0:
		return nil, errLoadBalancerServiceNotFound
	case count > 1:
		return nil, errors.New("more than one element found")
	}

	if err := l.annotationLoadbalancerServicePatch(service, lbService[0]); err != nil {
		return nil, err
	}

	return lbService[0], nil
}

func buildLoadBalancerWithAnnotations(service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	instancepoolID := getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, "")
	if instancepoolID == "" {
		return nil, fmt.Errorf("annotation %s is missing", annotationLoadBalancerServiceInstancePoolID)
	}

	servicePort, serviceTargetPort, err := getLoadBalancerServicePorts(service)
	if err != nil {
		return nil, err
	}

	hcPort, err := getLoadBalancerHealthCheckPort(service)
	if err != nil {
		return nil, err
	}

	hcInterval, err := time.ParseDuration(getAnnotation(service, annotationLoadBalancerServiceHealthCheckInterval, "10s"))
	if err != nil {
		return nil, err
	}

	hcTimeout, err := time.ParseDuration(getAnnotation(service, annotationLoadBalancerServiceHealthCheckTimeout, "2s"))
	if err != nil {
		return nil, err
	}

	hcRetries, err := strconv.Atoi(getAnnotation(service, annotationLoadBalancerServiceHealthCheckRetries, "1"))
	if err != nil {
		return nil, err
	}

	return &egoscale.NetworkLoadBalancer{
		ID:          getAnnotation(service, annotationLoadBalancerID, ""),
		Name:        getAnnotation(service, annotationLoadBalancerName, "nlb-"+string(service.UID)),
		Description: getAnnotation(service, annotationLoadBalancerDescription, "kubernetes load balancer "+service.Name),
		Services: func() []*egoscale.NetworkLoadBalancerService {
			services := make([]*egoscale.NetworkLoadBalancerService, 0)
			services = append(services, &egoscale.NetworkLoadBalancerService{
				ID:             getAnnotation(service, annotationLoadBalancerServiceID, ""),
				Name:           getAnnotation(service, annotationLoadBalancerServiceName, "nlb-service-"+string(service.UID)),
				Description:    getAnnotation(service, annotationLoadBalancerServiceDescription, "kubernetes load balancer "+service.Name),
				InstancePoolID: instancepoolID,
				Protocol:       getAnnotation(service, annotationLoadBalancerServiceProtocol, "tcp"),
				Port:           uint16(servicePort),
				TargetPort:     uint16(serviceTargetPort),
				Strategy:       getAnnotation(service, annotationLoadBalancerServiceStrategy, "round-robin"),
				Healthcheck: egoscale.NetworkLoadBalancerServiceHealthcheck{
					Mode:     getAnnotation(service, annotationLoadBalancerServiceHealthCheckMode, "tcp"),
					Port:     hcPort,
					Interval: hcInterval,
					Timeout:  hcTimeout,
					Retries:  int64(hcRetries),
					URI:      getAnnotation(service, annotationLoadBalancerServiceHealthCheckHTTPURI, "/"),
				},
			})

			return services
		}(),
	}, nil
}

func getAnnotation(service *v1.Service, annotation, defaultValue string) string {
	v, ok := service.Annotations[annotation]
	if !ok {
		return defaultValue
	}

	return v
}

func getLoadBalancerZone(service *v1.Service) (string, error) {
	zone, ok := service.Annotations[annotationLoadBalancerZone]
	if !ok {
		return "", errors.New("annotation " + annotationLoadBalancerZone + " is missing")
	}

	return zone, nil
}

func getLoadBalancerServicePorts(service *v1.Service) (int32, int32, error) {
	if len(service.Spec.Ports) == 1 {
		return service.Spec.Ports[0].Port, service.Spec.Ports[0].NodePort, nil
	}

	for _, port := range service.Spec.Ports {
		if port.Name == "service" {
			return port.Port, port.NodePort, nil
		}
	}

	return 0, 0, errors.New("specified service port does not exist")
}

func getLoadBalancerHealthCheckPort(service *v1.Service) (uint16, error) {
	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		return uint16(service.Spec.HealthCheckNodePort), nil
	}

	if len(service.Spec.Ports) == 1 {
		return uint16(service.Spec.Ports[0].NodePort), nil
	}

	for _, port := range service.Spec.Ports {
		if port.Name == "health-check" && port.Protocol == v1.ProtocolTCP {
			return uint16(port.NodePort), nil
		}
	}

	return 0, errors.New("specified health-check port does not exist or is not a TCP protocol")
}
