package exoscale

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/exoscale/egoscale"
	"gopkg.in/fsnotify.v1"
)

const defaultComputeEndpoint = "https://api.exoscale.com/v1"

type exoscaleClient struct {
	client          *egoscale.Client
	credentialsFile string
	endpoint        string
	*sync.RWMutex
}

type exoscaleAPICredentials struct {
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
	Name      string `json:"name"`
}

func newExoscaleClient(stop <-chan struct{}) (*exoscaleClient, error) {
	exoclient := &exoscaleClient{RWMutex: &sync.RWMutex{}, endpoint: defaultComputeEndpoint}

	envEndpoint := os.Getenv("EXOSCALE_API_ENDPOINT")
	envKey := os.Getenv("EXOSCALE_API_KEY")
	envSecret := os.Getenv("EXOSCALE_API_SECRET")
	envCredentialsFile := os.Getenv("EXOSCALE_API_CREDENTIALS_FILE")

	if envEndpoint != "" {
		exoclient.endpoint = envEndpoint
	}

	egoscale.UserAgent = fmt.Sprintf("Exoscale-K8s-Cloud-Controller/%s %s", versionString, egoscale.UserAgent)

	if envKey != "" && envSecret != "" {
		infof("reading exoscale IAM credentials from environment")
		exoclient.client = egoscale.NewClient(exoclient.endpoint, envKey, envSecret)
	} else if envCredentialsFile != "" {
		exoclient.credentialsFile = envCredentialsFile
		infof("reading exoscale IAM credentials from file %q", exoclient.credentialsFile)
		exoclient.refreshCredentials()
		go exoclient.watchCredentialsFile(stop)
	} else {
		return nil, errors.New("incomplete or missing for API credentials")
	}

	return exoclient, nil
}

func (e *exoscaleClient) watchCredentialsFile(stop <-chan struct{}) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fatalf("failed to watch credentials file %q, %q", e.credentialsFile, err)
	}

	// We watch the folder because the file might get deleted and recreated
	err = watcher.Add(filepath.Dir(e.credentialsFile))
	if err != nil {
		fatalf("failed to watch credentials file %q, %q", e.credentialsFile, err)
	}

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				errorf("credentials file watcher event channel closed")
				return
			}

			if event.Name == e.credentialsFile &&
				(event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create) {
				infof("refreshing IAM credential from file %s", e.credentialsFile)
				e.refreshCredentials()
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				errorf("credentials file watcher error channel closed")
				return
			}
			errorf("error while watching credentials file %q, %q", e.credentialsFile, err)
		case <-stop:
			infof("closing credential file watch")
			watcher.Close()
			return
		}
	}

}

func (e *exoscaleClient) refreshCredentials() {
	e.Lock()

	f, err := os.Open(e.credentialsFile)
	if err != nil {
		fatalf("failed to read credentials file %q: %s", e.credentialsFile, err)
	}

	var credentials exoscaleAPICredentials
	err = json.NewDecoder(f).Decode(&credentials)
	if err != nil {
		fatalf("failed to decode credentials file %q: %s", e.credentialsFile, err)
	}
	f.Close()

	e.client = egoscale.NewClient(e.endpoint, credentials.APIKey, credentials.APISecret)
	e.Unlock()

	infof("exoscale IAM credentials refreshed, now using %s (%s)", credentials.Name, credentials.APIKey)
}

func (e *exoscaleClient) CreateNetworkLoadBalancer(ctx context.Context, zone string,
	lbSpec *egoscale.NetworkLoadBalancer) (*egoscale.NetworkLoadBalancer, error) {
	e.RLock()
	defer e.RUnlock()

	return e.client.CreateNetworkLoadBalancer(ctx, zone, lbSpec)
}

func (e *exoscaleClient) DeleteNetworkLoadBalancer(ctx context.Context, zone, id string) error {
	e.RLock()
	defer e.RUnlock()

	return e.client.DeleteNetworkLoadBalancer(ctx, zone, id)
}

func (e *exoscaleClient) GetNetworkLoadBalancer(ctx context.Context, zone, id string) (*egoscale.NetworkLoadBalancer, error) {
	e.RLock()
	defer e.RUnlock()

	return e.client.GetNetworkLoadBalancer(ctx, zone, id)
}

func (e *exoscaleClient) UpdateNetworkLoadBalancer(ctx context.Context, zone string,
	nlbUpdate *egoscale.NetworkLoadBalancer) (*egoscale.NetworkLoadBalancer, error) {
	e.RLock()
	defer e.RUnlock()

	return e.client.UpdateNetworkLoadBalancer(ctx, zone, nlbUpdate)
}

func (e *exoscaleClient) GetVirtualMachine(ctx context.Context, uuid *egoscale.UUID) (*egoscale.VirtualMachine, error) {
	e.RLock()
	defer e.RUnlock()

	vm, err := e.client.GetWithContext(ctx, egoscale.VirtualMachine{ID: uuid})

	if vm == nil {
		return nil, err
	}

	return vm.(*egoscale.VirtualMachine), err
}
