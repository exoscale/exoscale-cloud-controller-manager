package exoscale

import (
	"fmt"
	"io"

	"github.com/exoscale/egoscale"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	version string
	commit  string

	versionString = fmt.Sprintf(
		"%s/%s", version, commit,
	)
)

const (
	providerName string = "exoscale"
)

type cloudProvider struct {
	client        *egoscale.Client
	instances     cloudprovider.Instances
	zones         cloudprovider.Zones
	loadbalancers cloudprovider.LoadBalancer
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(io.Reader) (cloudprovider.Interface, error) {
		return newExoscaleCloud()
	})
}

func newExoscaleCloud() (cloudprovider.Interface, error) {
	client, err := newExoscaleClient()
	if err != nil {
		return nil, fmt.Errorf("Could not create exoscale client: %#v", err)
	}

	return &cloudProvider{
		client:        client,
		instances:     newInstances(client),
		loadbalancers: newLoadBalancer(client),
		zones:         newZones(client),
	}, nil
}

func (c *cloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

// LoadBalancer returns a balancer interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, true
}

// Instances returns an instances interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Zones returns a zones interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) Zones() (cloudprovider.Zones, bool) {
	return c.zones, false
}

// Clusters is not implemented.
func (c *cloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes is not implemented.
func (c *cloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (c *cloudProvider) ProviderName() string {
	return providerName
}

// HasClusterID is not implemented.
func (c *cloudProvider) HasClusterID() bool {
	return false
}
