package exoscale

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	annotationLoadBalanceID                     = "service.beta.kubernetes.io/exo-lb-id"
	annotationLoadBalancerName                  = "service.beta.kubernetes.io/exo-lb-name"
	annotationLoadBalanceDescription            = "service.beta.kubernetes.io/exo-lb-description"
	annotationLoadBalanceZone                   = "service.beta.kubernetes.io/exo-lb-zone"
	serviceAnnotationLoadBalancerID             = "service.beta.kubernetes.io/exo-lb-service-id"
	serviceAnnotationLoadBalancerName           = "service.beta.kubernetes.io/exo-lb-service-name"
	serviceAnnotationLoadBalancerDescription    = "service.beta.kubernetes.io/exo-lb-service-description"
	serviceAnnotationLoadBalancerInstancePoolID = "service.beta.kubernetes.io/exo-lb-service-instancepoolid"
	// serviceAnnotationLoadBalancerProtocol            = "service.beta.kubernetes.io/exo-lb-service-protocol"
	serviceAnnotationLoadBalancerStrategy            = "service.beta.kubernetes.io/exo-lb-service-strategy"
	serviceAnnotationLoadBalancerHealthCheckPort     = "service.beta.kubernetes.io/exo-lb-service-health-check-port"
	serviceAnnotationLoadBalancerHealthCheckMode     = "service.beta.kubernetes.io/exo-lb-service-health-check-mode"
	serviceAnnotationLoadBalancerHealthCheckInterval = "service.beta.kubernetes.io/exo-lb-service-health-check-interval"
	serviceAnnotationLoadBalancerHealthCheckTimeout  = "service.beta.kubernetes.io/exo-lb-service-health-check-timeout"
	serviceAnnotationLoadBalancerHealthCheckRetries  = "service.beta.kubernetes.io/exo-lb-service-health-check-retries"
	serviceAnnotationLoadBalancerHealthCheckHTTPURI  = "service.beta.kubernetes.io/exo-lb-service-http-health-check-uri"
)

var (
	LoadBalancerNotFound       = errors.New("loadbalancer not found")
	LoadBalanceServiceNotFound = errors.New("loadbalancer service not found")
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
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return nil, false, err
	}

	lb, err := l.fetchLoadBalancer(ctx, zone, service)
	if err != nil {
		if err == LoadBalancerNotFound {
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
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return nil, err
	}

	lb, err := l.fetchLoadBalancer(ctx, zone, service)
	switch err {
	case nil:
		if err := l.updateLoadBalancer(ctx, zone, lb, service); err != nil {
			return nil, err
		}

	case LoadBalancerNotFound:
		lb, err = l.createLoadBalancer(ctx, zone, service)
		if err != nil {
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

	lb, err := l.fetchLoadBalancer(ctx, zone, service)
	if err != nil {
		return err
	}

	return l.updateLoadBalancer(ctx, zone, lb, service)
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
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return err
	}

	lb, err := l.fetchLoadBalancer(ctx, zone, service)
	if err != nil {
		if err == LoadBalancerNotFound {
			return nil
		}

		return err
	}

	return l.p.client.DeleteNetworkLoadBalancer(ctx, zone, lb.ID)
}

func (l *loadBalancer) createLoadBalancer(ctx context.Context, zone string, service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	lbName := l.GetLoadBalancerName(ctx, "", service)

	lbDescription := getLoadBalancerDescription(service)

	lb, err := l.p.client.CreateNetworkLoadBalancer(
		ctx,
		zone,
		&egoscale.NetworkLoadBalancer{
			Name:        lbName,
			Description: lbDescription,
		})
	if err != nil {
		return nil, err
	}

	if err := l.addLoadBalancerService(ctx, lb, service); err != nil {
		return nil, err
	}

	return lb, nil
}

func (l *loadBalancer) fetchLoadBalancer(ctx context.Context, zone string, service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	if lbID := getLoadBalancerID(service); lbID != "" {
		nlb, err := l.p.client.GetNetworkLoadBalancer(ctx, zone, lbID)
		if err != nil {
			if err == egoscale.ErrNotFound {
				return nil, LoadBalancerNotFound
			}

			return nil, err
		}

		return nlb, nil
	}

	return l.getLoadBalancerByName(ctx, zone, service)
}

func (l *loadBalancer) updateLoadBalancer(ctx context.Context, zone string, lb *egoscale.NetworkLoadBalancer, service *v1.Service) error {
	lb.Name = getLoadBalancerServiceName(service)
	lb.Description = getLoadBalancerDescription(service)

	_, err := l.p.client.UpdateNetworkLoadBalancer(ctx, zone, lb)
	if err != nil {
		return err
	}

	lbService, err := l.fetchLoadBalancerService(lb, service)
	if err != nil {
		if err == LoadBalanceServiceNotFound {
			return l.addLoadBalancerService(ctx, lb, service)
		}
		return err
	}

	return l.updateLoadBalancerService(ctx, lb, lbService.ID, service)
}

func (l *loadBalancer) getLoadBalancerByName(ctx context.Context, zone string, service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	name := l.GetLoadBalancerName(ctx, "", service)

	resp, err := l.p.client.ListNetworkLoadBalancers(ctx, zone)
	if err != nil {
		return nil, err
	}

	var loadbalancer []*egoscale.NetworkLoadBalancer
	for _, nlb := range resp {
		if nlb.Name == name {
			loadbalancer = append(loadbalancer, nlb)
		}
	}

	switch count := len(loadbalancer); {
	case count == 0:
		return nil, LoadBalancerNotFound
	case count > 1:
		return nil, errors.New("more than one element found")
	}

	return loadbalancer[0], nil
}

func getLoadBalancerZone(service *v1.Service) (string, error) {
	zone, ok := service.Annotations[annotationLoadBalanceZone]
	if !ok {
		return "", errors.New("annotation " + annotationLoadBalanceZone + " is missing")
	}

	return zone, nil
}

func getLoadBalancerID(service *v1.Service) string {
	lbID, ok := service.Annotations[annotationLoadBalanceID]
	if !ok {
		return ""
	}

	return lbID
}

func getLoadBalancerName(service *v1.Service) string {
	name, ok := service.Annotations[annotationLoadBalancerName]
	kubeName := string(service.UID)

	if ok {
		return name
	}

	return "nlb-" + kubeName
}

func getLoadBalancerPorts(service *v1.Service) (uint16, uint16, error) {
	port := service.Spec.Ports[0]

	if port.TargetPort.Type == intstr.String {
		return 0, 0, errors.New("TargetPort must be in the range 1 to 65535")
	}

	return uint16(port.Port), uint16(port.TargetPort.IntVal), nil
}

func getLoadBalancerDescription(service *v1.Service) string {
	description, ok := service.Annotations[annotationLoadBalanceDescription]
	if !ok {
		return "kubernetes load balancer " + service.Name
	}

	return description
}

func (l *loadBalancer) addLoadBalancerService(ctx context.Context, lb *egoscale.NetworkLoadBalancer, service *v1.Service) error {
	lbService, err := buildLoadBalancerService(service)
	if err != nil {
		return err
	}

	_, err = lb.AddService(ctx, lbService)
	if err != nil {
		return err
	}

	return nil
}

func (l *loadBalancer) fetchLoadBalancerService(lb *egoscale.NetworkLoadBalancer, service *v1.Service) (*egoscale.NetworkLoadBalancerService, error) {
	if serviceID := getLoadBalancerServiceID(service); serviceID != "" {
		for _, service := range lb.Services {
			if service.ID == serviceID {
				return service, nil
			}
		}
	}

	return getLoadBalancerServiceByName(lb, service)
}

func (l *loadBalancer) updateLoadBalancerService(ctx context.Context, lb *egoscale.NetworkLoadBalancer, serviceID string, service *v1.Service) error {
	lbService, err := buildLoadBalancerService(service)
	if err != nil {
		return err
	}

	lbService.ID = serviceID

	if err := lb.UpdateService(ctx, lbService); err != nil {
		return err
	}

	return nil
}

func buildLoadBalancerService(service *v1.Service) (*egoscale.NetworkLoadBalancerService, error) {
	serviceProtocol, err := getLoadBalancerServiceProtocol(service)
	if err != nil {
		return nil, err
	}

	servicePort, serviceTargetPort, err := getLoadBalancerPorts(service)
	if err != nil {
		return nil, err
	}

	hc, err := buildLoadBalancerServiceHealthCheck(service)
	if err != nil {
		return nil, err
	}

	return &egoscale.NetworkLoadBalancerService{
		Name:           getLoadBalancerServiceName(service),
		Description:    getLoadBalancerDescription(service),
		InstancePoolID: getLoadBalancerServiceInstancePoolID(service),
		Protocol:       serviceProtocol,
		Port:           int64(servicePort),
		TargetPort:     int64(serviceTargetPort),
		Strategy:       getLoadBalancerServiceStrategy(service),
		Healthcheck:    hc,
	}, nil
}

func getLoadBalancerServiceByName(lb *egoscale.NetworkLoadBalancer, service *v1.Service) (*egoscale.NetworkLoadBalancerService, error) {
	name := getLoadBalancerServiceName(service)

	var lbService []*egoscale.NetworkLoadBalancerService
	for _, service := range lb.Services {
		if service.Name == name {
			lbService = append(lbService, service)
		}
	}

	switch count := len(lbService); {
	case count == 0:
		return nil, LoadBalanceServiceNotFound
	case count > 1:
		return nil, errors.New("more than one element found")
	}

	return lbService[0], nil
}

func getLoadBalancerServiceID(service *v1.Service) string {
	serviceID, ok := service.Annotations[serviceAnnotationLoadBalancerID]
	if !ok {
		return ""
	}

	return serviceID
}

func getLoadBalancerServiceName(service *v1.Service) string {
	serviceName, ok := service.Annotations[serviceAnnotationLoadBalancerName]
	kubeName := string(service.UID)

	if !ok {
		return "nlb-service-" + kubeName
	}

	return serviceName
}

func getLoadBalancerServiceDescription(service *v1.Service) string {
	serviceDescription, ok := service.Annotations[serviceAnnotationLoadBalancerDescription]
	if !ok {
		return "kubernetes load balancer service " + service.Name
	}

	return serviceDescription
}

func getLoadBalancerServiceInstancePoolID(service *v1.Service) string {
	serviceID, ok := service.Annotations[serviceAnnotationLoadBalancerInstancePoolID]
	if !ok {
		return ""
	}

	return serviceID
}

func getLoadBalancerServiceProtocol(service *v1.Service) (string, error) {
	switch protocol := service.Spec.Ports[0].Protocol; protocol {
	case "SCTP":
		return "", errors.New("Only TCP or UDP Protocols are supported")
	case "UDP":
		return "udp", nil
	default:
		return "tcp", nil
	}
}

func getLoadBalancerServiceStrategy(service *v1.Service) string {
	return service.Annotations[serviceAnnotationLoadBalancerStrategy]
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
		Mode:     getLoadBalancerHealthCheckMode(service),
		Port:     int64(hcPort),
		URI:      getLoadBalancerHealthCheckURI(service),
		Interval: hcInterval,
		Timeout:  hcTimeout,
		Retries:  hcRetries,
	}, nil
}

func getLoadBalancerHealthCheckMode(service *v1.Service) string {
	return service.Annotations[serviceAnnotationLoadBalancerHealthCheckMode]
}

func getLoadBalancerHealthCheckInterval(service *v1.Service) (time.Duration, error) {
	hcInterval, ok := service.Annotations[serviceAnnotationLoadBalancerHealthCheckInterval]
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
	hcTimeout, ok := service.Annotations[serviceAnnotationLoadBalancerHealthCheckTimeout]
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
	hcRetries, ok := service.Annotations[serviceAnnotationLoadBalancerHealthCheckRetries]
	if !ok {
		return 1, nil
	}

	retries, err := strconv.Atoi(hcRetries)
	if err != nil {
		return 0, err
	}

	return int64(retries), nil
}

func getLoadBalancerHealthCheckURI(service *v1.Service) string {
	hcHTTPURI, ok := service.Annotations[serviceAnnotationLoadBalancerHealthCheckHTTPURI]
	if !ok {
		return "/"
	}
	return hcHTTPURI
}

func getLoadBalancerHealthCheckPort(service *v1.Service) (uint16, error) {
	hcPort, ok := service.Annotations[serviceAnnotationLoadBalancerHealthCheckPort]
	if !ok {
		return uint16(service.Spec.HealthCheckNodePort), nil
	}

	port, err := strconv.ParseUint(hcPort, 10, 16)
	if err != nil {
		return 0, err
	}

	return uint16(port), nil
}
