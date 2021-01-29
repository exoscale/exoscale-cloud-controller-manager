package exoscale

import (
	"fmt"
	"io"
	"os"

	"k8s.io/client-go/kubernetes"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	version string
	commit  string

	versionString = fmt.Sprintf("%s/%s", version, commit)
)

const (
	providerName   = "exoscale"
	providerPrefix = "exoscale://"
)

type cloudProvider struct {
	client       *exoscaleClient
	instances    cloudprovider.Instances
	zones        cloudprovider.Zones
	loadBalancer cloudprovider.LoadBalancer
	kclient      kubernetes.Interface
	defaultZone  string
	stop         <-chan struct{}
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(io.Reader) (cloudprovider.Interface, error) {
		return newExoscaleCloud()
	})
}

func newExoscaleCloud() (cloudprovider.Interface, error) {
	provider := &cloudProvider{}

	provider.instances = newInstances(provider)
	provider.loadBalancer = newLoadBalancer(provider)
	provider.zones = newZones(provider)
	provider.defaultZone = os.Getenv("EXOSCALE_DEFAULT_ZONE")

	return provider, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (c *cloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	restConfig := clientBuilder.ConfigOrDie("exoscale-cloud-controller-manager")
	c.kclient = kubernetes.NewForConfigOrDie(restConfig)

	client, err := newExoscaleClient(stop)
	if err != nil {
		fatalf("could not create Exoscale client: %v", err)
	}
	c.client = client
}

// LoadBalancer returns a balancer interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return c.loadBalancer, true
}

// Instances returns an instances interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) Instances() (cloudprovider.Instances, bool) {
	return c.instances, true
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface.
// Also returns true if the interface is supported, false otherwise.
func (c *cloudProvider) Zones() (cloudprovider.Zones, bool) {
	return c.zones, true
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
