package exoscale

import (
	"fmt"
	"io"

	"github.com/exoscale/egoscale"
	cloudprovider "k8s.io/cloud-provider"
)

const (
	// ProviderName specifies the name for the Exoscale provider
	providerName string = "exoscale"
)

// cloudProvider implents Instances, Zones, and LoadBalancer
type cloudProvider struct {
	client *egoscale.Client
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(config io.Reader) (cloudprovider.Interface, error) {
		// Config param is optional.
		// Can be possible to use it to load config (secrets...etc) from here:
		// https://pkg.go.dev/k8s.io/cloud-provider@v0.17.0?tab=doc#Factory
		return newExoscaleCloud(config)
	})
}

func newExoscaleCloud(_ io.Reader) (cloudprovider.Interface, error) {
	client, err := newExoscaleClient()
	if err != nil {
		return nil, fmt.Errorf("Could not create exoscale client: %#v", err)
	}

	return &cloudProvider{
		client: client,
		// instances:     newInstances(resources, region),
		// zones:         newZones(resources, region),
		// loadbalancers: newLoadBalancers(resources, doClient, region),
		// ...etc
	}, nil
}

func newExoscaleClient() (*egoscale.Client, error) {
	//TODO
	return nil, cloudprovider.NotImplemented
}

func (c *cloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	//TODO Initialize is not implemented.
}

// LoadBalancer is not implemented.
func (c *cloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	// example:
	// return c.loadbalancers, true
	return nil, false
}

// Instances is not implemented.
func (c *cloudProvider) Instances() (cloudprovider.Instances, bool) {
	// example:
	// return c.instances, true
	return nil, false
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
