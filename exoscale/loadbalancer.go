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
	// annotationLoadBalancerID is the ID of the loadbalancer
	annotationLoadBalancerID = "service.beta.kubernetes.io/exo-lb-id"

	// annotationLoadBalancerName is the name of the loadbalancer
	annotationLoadBalancerName = "service.beta.kubernetes.io/exo-lb-name"

	// annotationLoadBalancerDescription is the description of the loadbalancer
	annotationLoadBalancerDescription = "service.beta.kubernetes.io/exo-lb-description"

	// annotationLoadBalancerZone is the zone of the loadbalancer
	// the possible values are "bg-sof-1", "ch-dk-2", "ch-gva-2", "de-fra-1", "de-muc-1"
	annotationLoadBalancerZone = "service.beta.kubernetes.io/exo-lb-zone"

	// annotationLoadBalancerServiceID is the ID of the service associated to a loadbalancer
	annotationLoadBalancerServiceID = "service.beta.kubernetes.io/exo-lb-service-id"

	// annotationLoadBalancerServiceName is the name of the service associated to a loadbalancer
	annotationLoadBalancerServiceName = "service.beta.kubernetes.io/exo-lb-service-name"

	// annotationLoadBalancerServiceDescription is the description of the service associated to a loadbalancer
	annotationLoadBalancerServiceDescription = "service.beta.kubernetes.io/exo-lb-service-description"

	// annotationLoadBalancerServiceInstancePoolID is the ID of the instance pool associated to a service
	annotationLoadBalancerServiceInstancePoolID = "service.beta.kubernetes.io/exo-lb-service-instancepoolid"

	// annotationLoadBalancerServiceStrategy is the strategy of the service associated to a loadbalancer
	// the possible values are "round-robin" or "source-hash"
	annotationLoadBalancerServiceStrategy = "service.beta.kubernetes.io/exo-lb-service-strategy"

	// annotationLoadBalancerServiceProtocol is the protocol of the service associated to a loadbalancer
	// the possible values are "tcp" or "http"
	annotationLoadBalancerServiceProtocol = "service.beta.kubernetes.io/exo-lb-service-protocol"

	// annotationLoadBalancerServiceHealthCheckMode is the mode of health check
	// the default value is "tcp" and the value can be "http"
	annotationLoadBalancerServiceHealthCheckMode = "service.beta.kubernetes.io/exo-lb-service-health-check-mode"

	// annotationLoadBalancerServiceHealthCheckInterval is the interval between two consecutive health checks
	annotationLoadBalancerServiceHealthCheckInterval = "service.beta.kubernetes.io/exo-lb-service-health-check-interval"

	// annotationLoadBalancerServiceHealthCheckTimeout is the health check timeout
	annotationLoadBalancerServiceHealthCheckTimeout = "service.beta.kubernetes.io/exo-lb-service-health-check-timeout"

	// annotationLoadBalancerServiceHealthCheckRetries is number of retries before considering a service failed
	annotationLoadBalancerServiceHealthCheckRetries = "service.beta.kubernetes.io/exo-lb-service-health-check-retries"

	// annotationLoadBalancerServiceHealthCheckHTTPURI is the URI that is used by the "http" health check
	// the default value is "/"
	annotationLoadBalancerServiceHealthCheckHTTPURI = "service.beta.kubernetes.io/exo-lb-service-http-health-check-uri"
)

var (
	errLoadBalancerNotFound        = errors.New("loadbalancer not found")
	errLoadBalancerServiceNotFound = errors.New("loadbalancer service not found")
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
	return getLoadBalancerName(service)
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, _ []*v1.Node) (*v1.LoadBalancerStatus, error) {
	kubelb, err := buildLoadBalancerWithAnnotations(service)
	if err != nil {
		return nil, err
	}

	lb, zone, err := l.fetchLoadBalancer(ctx, service)
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

	if lbID := getLoadBalancerID(service); lbID != "" {
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
		if lb.Name == getLoadBalancerName(service) {
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
		if svc.ID == getLoadBalancerServiceID(service) {
			return svc, nil
		}
		if svc.Name == getLoadBalancerServiceName(service) {
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
	instancepoolID, err := getLoadBalancerServiceInstancePoolID(service)
	if err != nil {
		return nil, err
	}

	servicePort, serviceTargetPort, err := getLoadBalancerServicePorts(service)
	if err != nil {
		return nil, err
	}

	hc, err := buildLoadBalancerServiceHealthCheck(service)
	if err != nil {
		return nil, err
	}

	return &egoscale.NetworkLoadBalancer{
		ID:          getLoadBalancerID(service),
		Name:        getLoadBalancerName(service),
		Description: getLoadBalancerDescription(service),
		Services: func() []*egoscale.NetworkLoadBalancerService {
			services := make([]*egoscale.NetworkLoadBalancerService, 0)
			services = append(services, &egoscale.NetworkLoadBalancerService{
				ID:             getLoadBalancerServiceID(service),
				Name:           getLoadBalancerServiceName(service),
				Description:    getLoadBalancerServiceDescription(service),
				InstancePoolID: instancepoolID,
				Protocol:       getLoadBalancerServiceProtocol(service),
				Port:           uint16(servicePort),
				TargetPort:     uint16(serviceTargetPort),
				Strategy:       getLoadBalancerServiceStrategy(service),
				Healthcheck:    hc,
			})

			return services
		}(),
	}, nil
}

func buildLoadBalancerServiceHealthCheck(service *v1.Service) (egoscale.NetworkLoadBalancerServiceHealthcheck, error) {
	hcInterval, err := getLoadBalancerHealthCheckInterval(service)
	if err != nil {
		return egoscale.NetworkLoadBalancerServiceHealthcheck{}, err
	}

	hcTimeout, err := getLoadBalancerHealthCheckTimeout(service)
	if err != nil {
		return egoscale.NetworkLoadBalancerServiceHealthcheck{}, err
	}

	hcRetries, err := getLoadBalancerHealthCheckRetries(service)
	if err != nil {
		return egoscale.NetworkLoadBalancerServiceHealthcheck{}, err
	}

	hcPort, err := getLoadBalancerHealthCheckPort(service)
	if err != nil {
		return egoscale.NetworkLoadBalancerServiceHealthcheck{}, err
	}

	return egoscale.NetworkLoadBalancerServiceHealthcheck{
		Mode:     getLoadBalancerHealthCkeckMode(service),
		Port:     uint16(hcPort),
		URI:      getLoadBalancerHealthCheckURI(service),
		Interval: hcInterval,
		Timeout:  hcTimeout,
		Retries:  hcRetries,
	}, nil
}

func getLoadBalancerZone(service *v1.Service) (string, error) {
	zone, ok := service.Annotations[annotationLoadBalancerZone]
	if !ok {
		return "", errors.New("annotation " + annotationLoadBalancerZone + " is missing")
	}

	return zone, nil
}

func getLoadBalancerID(service *v1.Service) string {
	lbID, ok := service.Annotations[annotationLoadBalancerID]
	if !ok {
		return ""
	}

	return lbID
}

func getLoadBalancerName(service *v1.Service) string {
	name, ok := service.Annotations[annotationLoadBalancerName]
	kubeName := string(service.UID)

	if !ok {
		return "nlb-" + kubeName
	}

	return name
}

func getLoadBalancerDescription(service *v1.Service) string {
	description, ok := service.Annotations[annotationLoadBalancerDescription]
	if !ok {
		return "kubernetes load balancer " + service.Name
	}

	return description
}

func getLoadBalancerServiceID(service *v1.Service) string {
	serviceID, ok := service.Annotations[annotationLoadBalancerServiceID]
	if !ok {
		return ""
	}

	return serviceID
}

func getLoadBalancerServiceName(service *v1.Service) string {
	serviceName, ok := service.Annotations[annotationLoadBalancerServiceName]
	kubeName := string(service.UID)

	if !ok {
		return "nlb-service-" + kubeName
	}

	return serviceName
}

func getLoadBalancerServiceDescription(service *v1.Service) string {
	serviceDescription, ok := service.Annotations[annotationLoadBalancerServiceDescription]
	if !ok {
		return "kubernetes load balancer service " + service.Name
	}

	return serviceDescription
}

func getLoadBalancerServiceInstancePoolID(service *v1.Service) (string, error) {
	instancepoolID, ok := service.Annotations[annotationLoadBalancerServiceInstancePoolID]
	if !ok {
		return "", fmt.Errorf("annotation %s is missing", annotationLoadBalancerServiceInstancePoolID)
	}

	return instancepoolID, nil
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

func getLoadBalancerServiceProtocol(service *v1.Service) string {
	protocol, ok := service.Annotations[annotationLoadBalancerServiceProtocol]
	if !ok {
		return "tcp"
	}

	return protocol
}

func getLoadBalancerServiceStrategy(service *v1.Service) string {
	strategy, ok := service.Annotations[annotationLoadBalancerServiceStrategy]
	if !ok {
		return "round-robin"
	}

	return strategy
}

func getLoadBalancerHealthCheckInterval(service *v1.Service) (time.Duration, error) {
	hcInterval, ok := service.Annotations[annotationLoadBalancerServiceHealthCheckInterval]
	if !ok {
		return time.ParseDuration("10s")
	}

	hcDuration, err := time.ParseDuration(hcInterval)
	if err != nil {
		return time.Duration(0), err
	}

	return hcDuration, nil
}

func getLoadBalancerHealthCheckTimeout(service *v1.Service) (time.Duration, error) {
	hcTimeout, ok := service.Annotations[annotationLoadBalancerServiceHealthCheckTimeout]
	if !ok {
		return time.ParseDuration("2s")
	}

	hcDuration, err := time.ParseDuration(hcTimeout)
	if err != nil {
		return time.Duration(0), err
	}

	return hcDuration, nil
}

func getLoadBalancerHealthCheckRetries(service *v1.Service) (int64, error) {
	hcRetries, ok := service.Annotations[annotationLoadBalancerServiceHealthCheckRetries]
	if !ok {
		return 1, nil
	}

	retries, err := strconv.Atoi(hcRetries)
	if err != nil {
		return 0, err
	}

	return int64(retries), nil
}

func getLoadBalancerHealthCkeckMode(service *v1.Service) string {
	protocol, ok := service.Annotations[annotationLoadBalancerServiceHealthCheckMode]
	if !ok {
		return "tcp"
	}

	return protocol
}

func getLoadBalancerHealthCheckURI(service *v1.Service) string {
	hcHTTPURI, ok := service.Annotations[annotationLoadBalancerServiceHealthCheckHTTPURI]
	if !ok {
		return "/"
	}
	return hcHTTPURI
}

func getLoadBalancerHealthCheckPort(service *v1.Service) (int32, error) {
	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		return service.Spec.HealthCheckNodePort, nil
	}

	if len(service.Spec.Ports) == 1 {
		return service.Spec.Ports[0].NodePort, nil
	}

	for _, port := range service.Spec.Ports {
		if port.Name == "health-check" && port.Protocol == v1.ProtocolTCP {
			return port.NodePort, nil
		}
	}

	return 0, errors.New("specified health-check port does not exist or is not a TCP protocol")
}
