package exoscale

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	egoscale "github.com/exoscale/egoscale/v2"
	"gopkg.in/fsnotify.v1"
)

const defaultComputeEnvironment = "api"

type exoscaleClient interface {
	CreateNetworkLoadBalancer(context.Context, string, *egoscale.NetworkLoadBalancer) (*egoscale.NetworkLoadBalancer, error)
	CreateNetworkLoadBalancerService(context.Context, string, *egoscale.NetworkLoadBalancer, *egoscale.NetworkLoadBalancerService) (*egoscale.NetworkLoadBalancerService, error)
	DeleteNetworkLoadBalancer(context.Context, string, *egoscale.NetworkLoadBalancer) error
	DeleteNetworkLoadBalancerService(context.Context, string, *egoscale.NetworkLoadBalancer, *egoscale.NetworkLoadBalancerService) error
	GetInstance(context.Context, string, string) (*egoscale.Instance, error)
	GetInstanceType(context.Context, string, string) (*egoscale.InstanceType, error)
	GetNetworkLoadBalancer(context.Context, string, string) (*egoscale.NetworkLoadBalancer, error)
	ListInstances(context.Context, string) ([]*egoscale.Instance, error)
	UpdateNetworkLoadBalancer(context.Context, string, *egoscale.NetworkLoadBalancer) error
	UpdateNetworkLoadBalancerService(context.Context, string, *egoscale.NetworkLoadBalancer, *egoscale.NetworkLoadBalancerService) error
}

type exoscaleAPICredentials struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	Name      string `json:"name"`
}

type refreshableExoscaleClient struct {
	exo            exoscaleClient
	apiCredentials exoscaleAPICredentials
	apiEnvironment string

	*sync.RWMutex
}

func newRefreshableExoscaleClient(ctx context.Context) (*refreshableExoscaleClient, error) {
	c := &refreshableExoscaleClient{
		RWMutex:        &sync.RWMutex{},
		apiEnvironment: defaultComputeEnvironment,
	}

	envAPIKey := os.Getenv("EXOSCALE_API_KEY")
	envAPISecret := os.Getenv("EXOSCALE_API_SECRET")
	envAPICredentialsFile := os.Getenv("EXOSCALE_API_CREDENTIALS_FILE")
	envAPIEnvironment := os.Getenv("EXOSCALE_API_ENVIRONMENT")

	if envAPIEnvironment != "" {
		c.apiEnvironment = envAPIEnvironment
	}

	if envAPIKey != "" && envAPISecret != "" {
		infof("reading Exoscale API credentials from environment")

		c.apiCredentials = exoscaleAPICredentials{
			APIKey:    envAPIKey,
			APISecret: envAPISecret,
		}

		exo, err := egoscale.NewClient(c.apiCredentials.APIKey, c.apiCredentials.APISecret)
		if err != nil {
			return nil, err
		}
		c.exo = exo
	} else if envAPICredentialsFile != "" {
		infof("reading Exoscale API credentials from file %q", envAPICredentialsFile)

		c.refreshCredentialsFromFile(envAPICredentialsFile)
		go c.watchCredentialsFile(ctx, envAPICredentialsFile)
	} else {
		return nil, errors.New("incomplete or missing Exoscale API credentials")
	}

	return c, nil
}

func (c *refreshableExoscaleClient) watchCredentialsFile(ctx context.Context, path string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fatalf("failed to watch credentials file %q: %v", path, err)
	}

	// We watch the folder because the file might get deleted and recreated.
	err = watcher.Add(filepath.Dir(path))
	if err != nil {
		fatalf("failed to watch credentials file %q: %v", path, err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				errorf("credentials file watcher event channel closed")
				return
			}

			if event.Name == path &&
				(event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				infof("refreshing API credentials from file %q", path)
				c.refreshCredentialsFromFile(path)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				errorf("credentials file watcher error channel closed")
				return
			}
			errorf("error while watching credentials file %q: %v", path, err)

		case <-ctx.Done():
			infof("closing credentials file watcher")
			_ = watcher.Close()
			return
		}
	}
}

func (c *refreshableExoscaleClient) refreshCredentialsFromFile(path string) {
	c.Lock()
	defer c.Unlock()

	f, err := os.Open(path)
	if err != nil {
		fatalf("failed to read credentials file %q: %v", path, err)
	}

	if err = json.NewDecoder(f).Decode(&c.apiCredentials); err != nil {
		fatalf("failed to decode credentials file %q: %v", path, err)
	}
	_ = f.Close()

	c.exo, err = egoscale.NewClient(c.apiCredentials.APIKey, c.apiCredentials.APISecret)
	if err != nil {
		fatalf("failed to initialize Exoscale client: %v", err)
	}

	infof(
		"Exoscale API credentials refreshed, now using %s (%s)",
		c.apiCredentials.Name,
		c.apiCredentials.APIKey,
	)
}
