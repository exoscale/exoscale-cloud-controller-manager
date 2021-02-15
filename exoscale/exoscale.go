package exoscale

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

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
	ctx          context.Context
	client       *exoscaleClient
	instances    cloudprovider.Instances
	zones        cloudprovider.Zones
	loadBalancer cloudprovider.LoadBalancer
	kclient      kubernetes.Interface
	zone         string

	stop func()
}

func init() {
	cloudprovider.RegisterCloudProvider(providerName, func(io.Reader) (cloudprovider.Interface, error) {
		return newExoscaleCloud()
	})
}

func newExoscaleCloud() (cloudprovider.Interface, error) {
	provider := &cloudProvider{
		zone: os.Getenv("EXOSCALE_ZONE"),
	}

	if provider.zone == "" {
		return nil, errors.New("zone not specified, please set the EXOSCALE_ZONE environment variable")
	}

	provider.instances = newInstances(provider)
	provider.loadBalancer = newLoadBalancer(provider)
	provider.zones = newZones(provider)

	return provider, nil
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping or run custom controllers specific to the cloud provider.
// Any tasks started here should be cleaned up when the stop channel closes.
func (p *cloudProvider) Initialize(clientBuilder cloudprovider.ControllerClientBuilder, stop <-chan struct{}) {
	restConfig := clientBuilder.ConfigOrDie("exoscale-cloud-controller-manager")
	p.kclient = kubernetes.NewForConfigOrDie(restConfig)

	ctx, cancel := context.WithCancel(context.Background())
	p.ctx = ctx
	p.stop = cancel

	client, err := newExoscaleClient(p.ctx)
	if err != nil {
		fatalf("could not create Exoscale client: %v", err)
	}
	p.client = client

	// Broadcast the upstream stop signal to all provider-level goroutines
	// watching the provider's context for cancellation.
	go func(provider *cloudProvider) {
		<-stop
		debugf("received cloud provider termination signal")
		provider.stop()
	}(p)

	if v := os.Getenv("EXOSCALE_SKS_AGENT_RUNNERS"); v != "" {
		if err := p.runSKSAgent(strings.Split(v, ",")); err != nil {
			fatalf("SKS agent failed to start: %s", err)
		}
	}
}

// LoadBalancer returns a balancer interface.
// Also returns true if the interface is supported, false otherwise.
func (p *cloudProvider) LoadBalancer() (cloudprovider.LoadBalancer, bool) {
	return p.loadBalancer, true
}

// Instances returns an instances interface.
// Also returns true if the interface is supported, false otherwise.
func (p *cloudProvider) Instances() (cloudprovider.Instances, bool) {
	return p.instances, true
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Implementing InstancesV2 is behaviorally identical to Instances but is optimized to significantly reduce
// API calls to the cloud provider when registering and syncing nodes.
// Also returns true if the interface is supported, false otherwise.
func (p *cloudProvider) InstancesV2() (cloudprovider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface.
// Also returns true if the interface is supported, false otherwise.
func (p *cloudProvider) Zones() (cloudprovider.Zones, bool) {
	return p.zones, true
}

// Clusters is not implemented.
func (p *cloudProvider) Clusters() (cloudprovider.Clusters, bool) {
	return nil, false
}

// Routes is not implemented.
func (p *cloudProvider) Routes() (cloudprovider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (p *cloudProvider) ProviderName() string {
	return providerName
}

// HasClusterID is not implemented.
func (p *cloudProvider) HasClusterID() bool {
	return false
}
