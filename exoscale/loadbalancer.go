package exoscale

import (
	"context"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	cloudprovider "k8s.io/cloud-provider"
)

type loadbalancers struct {
	client *egoscale.Client
	// can handle more variable
}

func newLoadBalancers(client *egoscale.Client) cloudprovider.LoadBalancer {
	return &loadbalancers{
		client: client,
	}
}

// TODO: Break this up into different interfaces (LB, etc) when we have more than one type of service
// GetLoadBalancer returns whether the specified load balancer exists, and
// if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadbalancers) GetLoadBalancer(ctx context.Context, clusterName string, service *v1.Service) (status *v1.LoadBalancerStatus, exists bool, err error) {
	return nil, false, cloudprovider.NotImplemented
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the
// *v1.Service parameter as read-only and not modify it.
func (l *loadbalancers) GetLoadBalancerName(ctx context.Context, clusterName string, service *v1.Service) string {
	return ""
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the balancer
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadbalancers) EnsureLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) (*v1.LoadBalancerStatus, error) {
	return nil, cloudprovider.NotImplemented
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and *v1.Node
// parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadbalancers) UpdateLoadBalancer(ctx context.Context, clusterName string, service *v1.Service, nodes []*v1.Node) error {
	return cloudprovider.NotImplemented
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it
// exists, returning nil if the load balancer specified either didn't exist or
// was successfully deleted.
// This construction is useful because many cloud providers' load balancers
// have multiple underlying components, meaning a Get could say that the LB
// doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to kube-controller-manager
func (l *loadbalancers) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *v1.Service) error {
	return cloudprovider.NotImplemented
}
