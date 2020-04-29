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
	// ProviderName specifies the name for the Exoscale provider
	providerName string = "exoscale"
)

// cloudProvider implents Instances, Zones, and LoadBalancer
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
		loadbalancers: newLoadBalancers(client),
		// zones:         newZones(client),
		// ...etc
	}, nil
}

func (c *cloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
}

func (c *cloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadbalancers, true
}

func (c *cloudProvider) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// Zones is not implemented.
func (c *cloudProvider) Zones() (cloudprovider.Zones, bool) {
	return nil, false
}

// clusters is not implemented.
func (c *cloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// routes is not implemented.
func (c *cloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

func (c *cloudProvider) ProviderName() string {
	return providerName
}

// HasClusterID is not implemented.
func (c *cloudProvider) HasClusterID() bool {
	return false
}
