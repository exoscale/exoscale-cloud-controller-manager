package exoscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"

	"gopkg.in/fsnotify.v1"
)

type exoscaleClient interface {
	CreateLoadBalancer(ctx context.Context, req v3.CreateLoadBalancerRequest) (*v3.Operation, error)
	AddServiceToLoadBalancer(ctx context.Context, id v3.UUID, req v3.AddServiceToLoadBalancerRequest) (*v3.Operation, error)
	DeleteLoadBalancer(ctx context.Context, id v3.UUID) (*v3.Operation, error)
	DeleteLoadBalancerService(ctx context.Context, id v3.UUID, serviceID v3.UUID) (*v3.Operation, error)
	GetInstance(ctx context.Context, id v3.UUID) (*v3.Instance, error)
	GetInstanceType(ctx context.Context, id v3.UUID) (*v3.InstanceType, error)
	GetLoadBalancer(ctx context.Context, id v3.UUID) (*v3.LoadBalancer, error)
	ListInstances(ctx context.Context, opts ...v3.ListInstancesOpt) (*v3.ListInstancesResponse, error)
	UpdateLoadBalancer(ctx context.Context, id v3.UUID, req v3.UpdateLoadBalancerRequest) (*v3.Operation, error)
	UpdateLoadBalancerService(ctx context.Context, id v3.UUID, serviceID v3.UUID, req v3.UpdateLoadBalancerServiceRequest) (*v3.Operation, error)
	Wait(ctx context.Context, op *v3.Operation, states ...v3.OperationState) (*v3.Operation, error)
}

type exoscaleAPICredentials struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	Name      string `json:"name"`
}

type refreshableExoscaleClient struct {
	exo            exoscaleClient
	apiCredentials exoscaleAPICredentials
	apiEndpoint    string

	*sync.RWMutex
}

func newRefreshableExoscaleClient(ctx context.Context, config *globalConfig) (*refreshableExoscaleClient, error) {
	c := &refreshableExoscaleClient{
		RWMutex: &sync.RWMutex{},
	}

	if config.APIEndpoint != "" {
		c.apiEndpoint = config.APIEndpoint
	}

	if config.APIKey != "" && config.APISecret != "" { //nolint:gocritic
		infof("using Exoscale actual API credentials (key + secret)")

		c.apiCredentials = exoscaleAPICredentials{
			APIKey:    config.APIKey,
			APISecret: config.APISecret,
		}

		//TODO add chain credentials with env...etc
		creds := credentials.NewStaticCredentials(c.apiCredentials.APIKey, c.apiCredentials.APISecret)
		exo, err := v3.NewClient(creds, v3.ClientOptWithUserAgent(
			fmt.Sprintf("Exoscale-K8s-Cloud-Controller/%s", versionString),
		))
		if err != nil {
			return nil, err
		}
		c.exo = exo
	} else if config.APICredentialsFile != "" {
		infof("reading (watching) Exoscale API credentials from file %q", config.APICredentialsFile)

		c.refreshCredentialsFromFile(config.APICredentialsFile)
		go c.watchCredentialsFile(ctx, config.APICredentialsFile)
	} else {
		return nil, errors.New("incomplete or missing Exoscale API credentials")
	}

	return c, nil
}

func (c *refreshableExoscaleClient) Wait(ctx context.Context, op *v3.Operation, states ...v3.OperationState) (*v3.Operation, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.Wait(
		ctx,
		op,
		states...,
	)
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
	f, err := os.Open(path)
	if err != nil {
		fatalf("failed to read credentials file %q: %v", path, err)
	}
	defer f.Close()

	var apiCredentials exoscaleAPICredentials
	if err = json.NewDecoder(f).Decode(&apiCredentials); err != nil {
		infof("failed to decode credentials file %q: %v", path, err)
		return
	}

	//TODO add chain credentials with env...etc
	creds := credentials.NewStaticCredentials(apiCredentials.APIKey, apiCredentials.APISecret)
	client, err := v3.NewClient(creds)
	if err != nil {
		infof("failed to initialize Exoscale client: %v", err)
		return
	}

	c.Lock()
	c.exo = client
	c.apiCredentials = apiCredentials
	c.Unlock()

	infof(
		"Exoscale API credentials refreshed, now using %s (%s)",
		c.apiCredentials.Name,
		c.apiCredentials.APIKey,
	)
}
