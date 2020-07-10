package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	annotationLoadBalancerZone                       = "service.beta.kubernetes.io/exoscale-loadbalancer-zone"
	annotationLoadBalancerID                         = "service.beta.kubernetes.io/exoscale-loadbalancer-id"
	annotationLoadBalancerName                       = "service.beta.kubernetes.io/exoscale-loadbalancer-name"
	annotationLoadBalancerDescription                = "service.beta.kubernetes.io/exoscale-loadbalancer-description"
	annotationLoadBalancerServiceStrategy            = "service.beta.kubernetes.io/exoscale-loadbalancer-service-strategy"
	annotationLoadBalancerServiceID                  = "service.beta.kubernetes.io/exoscale-loadbalancer-service-id"
	annotationLoadBalancerServiceName                = "service.beta.kubernetes.io/exoscale-loadbalancer-service-name"
	annotationLoadBalancerServiceDescription         = "service.beta.kubernetes.io/exoscale-loadbalancer-service-description"
	annotationLoadBalancerServiceInstancePoolID      = "service.beta.kubernetes.io/exoscale-loadbalancer-service-instancepool-id"
	annotationLoadBalancerServiceHealthCheckMode     = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-mode"
	annotationLoadBalancerServiceHealthCheckURI      = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-uri"
	annotationLoadBalancerServiceHealthCheckInterval = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-interval"
	annotationLoadBalancerServiceHealthCheckTimeout  = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-timeout"
	annotationLoadBalancerServiceHealthCheckRetries  = "service.beta.kubernetes.io/exoscale-loadbalancer-service-healthcheck-retries"

	servicePortNLBSvcPort            = "service"
	servicePortNLBSvcHealthcheckPort = "healthcheck"
)

var (
	errLoadBalancerNotFound        = errors.New("load balancer not found")
	errLoadBalancerServiceNotFound = errors.New("load balancer service not found")
)

type loadBalancer struct {
	p *cloudProvider
}

func newLoadBalancer(provider *cloudProvider) cloudprovider.LoadBalancer {
	return &loadBalancer{p: provider}
}

// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) GetLoadBalancer(ctx context.Context, _ string, service *v1.Service) (*v1.LoadBalancerStatus, bool, error) {
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return nil, false, err
	}

	lb, err := l.fetchLoadBalancer(ctx, service, zone)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil, false, nil
		}
		return nil, false, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb.IPAddress.String()}}}, true, nil
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadBalancer) GetLoadBalancerName(_ context.Context, _ string, service *v1.Service) string {
	return getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID))
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) EnsureLoadBalancer(ctx context.Context, _ string, service *v1.Service, _ []*v1.Node) (*v1.LoadBalancerStatus, error) {
	var lb *egoscale.NetworkLoadBalancer

	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return nil, err
	}

	lbDef, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return nil, err
	}

	_, err = l.fetchLoadBalancer(ctx, service, zone)
	switch err {
	case nil:
		lb, err = l.p.client.UpdateNetworkLoadBalancer(ctx, zone, lbDef)
		if err != nil {
			return nil, err
		}

	case errLoadBalancerNotFound:
		lb, err = l.p.client.CreateNetworkLoadBalancer(ctx, zone, lbDef)
		if err != nil {
			return nil, err
		}

		if err := l.patchLoadBalancerAnnotations(ctx, service, lb); err != nil {
			return nil, err
		}

	default:
		return nil, err
	}

	_, err = l.fetchLoadBalancerService(lb, service)
	switch err {
	case nil:
		if err := lb.UpdateService(ctx, lbDef.Services[0]); err != nil {
			return nil, err
		}

	case errLoadBalancerServiceNotFound:
		lbService, err := lb.AddService(ctx, lbDef.Services[0])
		if err != nil {
			return nil, err
		}

		if err := l.patchLoadBalancerServiceAnnotations(ctx, service, lbService); err != nil {
			return nil, err
		}

	default:
		return nil, err
	}

	return &v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: lb.IPAddress.String()}}}, nil
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadBalancer) UpdateLoadBalancer(ctx context.Context, _ string, service *v1.Service, _ []*v1.Node) error {
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return err
	}

	lbDef, err := buildLoadBalancerFromAnnotations(service)
	if err != nil {
		return err
	}

	lb, err := l.p.client.UpdateNetworkLoadBalancer(ctx, zone, lbDef)
	if err != nil {
		return err
	}
	if err := lb.UpdateService(ctx, lbDef.Services[0]); err != nil {
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
func (l *loadBalancer) EnsureLoadBalancerDeleted(ctx context.Context, _ string, service *v1.Service) error {
	zone, err := getLoadBalancerZone(service)
	if err != nil {
		return err
	}

	lb, err := l.fetchLoadBalancer(ctx, service, zone)
	if err != nil {
		if err == errLoadBalancerNotFound {
			return nil
		}

		return err
	}

	// We delete the NLB instance only if there is only one NLB service left (i.e. the one we're asked to delete).
	if len(lb.Services) == 1 {
		return l.p.client.DeleteNetworkLoadBalancer(ctx, zone, lb.ID)
	}

	// Otherwise, we only delete the NLB service and keep the NLB instance as it holds other services.
	lbService, err := l.fetchLoadBalancerService(lb, service)
	if err != nil {
		if err == errLoadBalancerServiceNotFound {
			return nil
		}

		return err
	}

	return lb.DeleteService(ctx, lbService)
}

func (l *loadBalancer) fetchLoadBalancer(ctx context.Context, service *v1.Service, zone string) (*egoscale.NetworkLoadBalancer, error) {
	if lbID := getAnnotation(service, annotationLoadBalancerID, ""); lbID != "" {
		lb, err := l.p.client.GetNetworkLoadBalancer(ctx, zone, lbID)
		switch err {
		case nil:
			return lb, nil

		case egoscale.ErrNotFound:
			return nil, errLoadBalancerNotFound

		default:
			return nil, err
		}
	}

	resp, err := l.p.client.ListNetworkLoadBalancers(ctx, zone)
	if err != nil {
		return nil, err
	}

	var loadbalancer *egoscale.NetworkLoadBalancer
	for _, lb := range resp {
		if lb.Name == getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID)) {
			loadbalancer = lb
		}
	}

	if loadbalancer == nil {
		return nil, errLoadBalancerNotFound
	}

	if err := l.patchLoadBalancerAnnotations(ctx, service, loadbalancer); err != nil {
		return nil, err
	}

	return loadbalancer, nil
}

func (l *loadBalancer) fetchLoadBalancerService(lb *egoscale.NetworkLoadBalancer, service *v1.Service) (*egoscale.NetworkLoadBalancerService, error) {
	ports, err := getLoadBalancerServicePorts(service)
	if err != nil {
		return nil, err
	}
	defaultServiceName := fmt.Sprintf("%s-%d", service.UID, ports[0])

	for _, svc := range lb.Services {
		if svc.ID == getAnnotation(service, annotationLoadBalancerServiceID, "") ||
			svc.Name == getAnnotation(service, annotationLoadBalancerServiceName, defaultServiceName) {
			return svc, nil
		}
	}

	return nil, errLoadBalancerServiceNotFound
}

func (l *loadBalancer) patchLoadBalancerAnnotations(ctx context.Context, service *v1.Service, lb *egoscale.NetworkLoadBalancer) error {
	patcher := newServicePatcher(ctx, l.p.kclient, service)

	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationLoadBalancerID] = lb.ID

	return patcher.Patch()
}

func (l *loadBalancer) patchLoadBalancerServiceAnnotations(ctx context.Context, service *v1.Service, lbService *egoscale.NetworkLoadBalancerService) error {
	patcher := newServicePatcher(ctx, l.p.kclient, service)

	if service.ObjectMeta.Annotations == nil {
		service.ObjectMeta.Annotations = map[string]string{}
	}
	service.ObjectMeta.Annotations[annotationLoadBalancerServiceID] = lbService.ID

	return patcher.Patch()
}

func buildLoadBalancerFromAnnotations(service *v1.Service) (*egoscale.NetworkLoadBalancer, error) {
	instancepoolID := getAnnotation(service, annotationLoadBalancerServiceInstancePoolID, "")
	if instancepoolID == "" {
		return nil, fmt.Errorf("annotation %s is missing", annotationLoadBalancerServiceInstancePoolID)
	}

	serviceProtocol, err := getLoadBalancerServiceProtocol(service)
	if err != nil {
		return nil, err
	}

	ports, err := getLoadBalancerServicePorts(service)
	if err != nil {
		return nil, err
	}
	servicePort, serviceTargetPort, hcPort := ports[0], ports[1], ports[2]

	hcInterval, err := time.ParseDuration(getAnnotation(service, annotationLoadBalancerServiceHealthCheckInterval, "10s"))
	if err != nil {
		return nil, err
	}

	hcTimeout, err := time.ParseDuration(getAnnotation(service, annotationLoadBalancerServiceHealthCheckTimeout, "5s"))
	if err != nil {
		return nil, err
	}

	hcRetries, err := strconv.Atoi(getAnnotation(service, annotationLoadBalancerServiceHealthCheckRetries, "1"))
	if err != nil {
		return nil, err
	}

	return &egoscale.NetworkLoadBalancer{
		ID:          getAnnotation(service, annotationLoadBalancerID, ""),
		Name:        getAnnotation(service, annotationLoadBalancerName, "k8s-"+string(service.UID)),
		Description: getAnnotation(service, annotationLoadBalancerDescription, ""),
		Services: []*egoscale.NetworkLoadBalancerService{{
			ID: getAnnotation(service, annotationLoadBalancerServiceID, ""),
			Name: getAnnotation(service, annotationLoadBalancerServiceName,
				fmt.Sprintf("%s-%d", service.UID, servicePort)),
			Description:    getAnnotation(service, annotationLoadBalancerServiceDescription, ""),
			InstancePoolID: instancepoolID,
			Protocol:       serviceProtocol,
			Port:           uint16(servicePort),
			TargetPort:     uint16(serviceTargetPort),
			Strategy:       getAnnotation(service, annotationLoadBalancerServiceStrategy, "round-robin"),
			Healthcheck: egoscale.NetworkLoadBalancerServiceHealthcheck{
				Mode:     getAnnotation(service, annotationLoadBalancerServiceHealthCheckMode, "tcp"),
				Port:     uint16(hcPort),
				Interval: hcInterval,
				Timeout:  hcTimeout,
				Retries:  int64(hcRetries),
				URI:      getAnnotation(service, annotationLoadBalancerServiceHealthCheckURI, "/"),
			},
		}},
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

// getLoadBalancerServicePorts returns the NLB service ports as a 3-tuple (service port/target port/healthcheck port)
// from a Kubernetes Service's specs.
func getLoadBalancerServicePorts(service *v1.Service) ([3]int32, error) {
	var servicePort, targetPort, healthcheckPort int32

	if len(service.Spec.Ports) == 1 {
		// If the service spec defines only one ServicePort, we infer the
		// NLB service healthcheck port from the ServicePort.NodePort.
		servicePort = service.Spec.Ports[0].Port
		targetPort = service.Spec.Ports[0].NodePort
		healthcheckPort = targetPort
	} else {
		// If the service spec defines more than one ServicePort, we look for
		// 2 named ServicePort:
		//   * "service" for the NLB service port and target port
		//   * "healthcheck" for the NLB healthcheck port
		for _, port := range service.Spec.Ports {
			switch port.Name {
			case servicePortNLBSvcPort:
				servicePort = port.Port
				targetPort = port.NodePort

			case servicePortNLBSvcHealthcheckPort:
				if port.Protocol != v1.ProtocolTCP {
					return [3]int32{}, errors.New("only TCP is supported as healthcheck port protocol")
				}
				healthcheckPort = port.NodePort
			}
		}

		switch {
		case servicePort == 0:
			return [3]int32{}, errors.New("service port not specified")

		case healthcheckPort == 0:
			return [3]int32{}, errors.New("service healthcheck port not specified")
		}
	}

	if service.Spec.ExternalTrafficPolicy == v1.ServiceExternalTrafficPolicyTypeLocal {
		healthcheckPort = service.Spec.HealthCheckNodePort
	}

	return [3]int32{servicePort, targetPort, healthcheckPort}, nil
}

func getLoadBalancerServiceProtocol(service *v1.Service) (string, error) {
	var protocol v1.Protocol

	if len(service.Spec.Ports) == 1 {
		protocol = service.Spec.Ports[0].Protocol
	} else {
		for _, port := range service.Spec.Ports {
			if port.Name == servicePortNLBSvcPort {
				protocol = port.Protocol
				break
			}
		}

	}

	// Exoscale NLB services can forward both TCP and UDP protocol, however the only supported
	// healthcheck protocol is TCP (plain TCP or HTTP).
	// Due to a technical limitation in Kubernetes preventing declaration of mixed protocols in a
	// service of type LoadBalancer (https://github.com/kubernetes/kubernetes/issues/23880) we only
	// allow TCP for service ports.
	if protocol != v1.ProtocolTCP {
		return "", errors.New("only TCP is supported as service port protocol")
	}

	return strings.ToLower(string(protocol)), nil
}
