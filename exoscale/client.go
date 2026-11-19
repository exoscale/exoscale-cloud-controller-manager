package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/klog/v2"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/exoscale/egoscale/v3/credentials"
)

type exoscaleClient interface {
	CreateLoadBalancer(ctx context.Context, req v3.CreateLoadBalancerRequest) (*v3.Operation, error)
	AddServiceToLoadBalancer(ctx context.Context, id v3.UUID, req v3.AddServiceToLoadBalancerRequest) (*v3.Operation, error)
	DeleteLoadBalancer(ctx context.Context, id v3.UUID) (*v3.Operation, error)
	DeleteLoadBalancerService(ctx context.Context, id v3.UUID, serviceID v3.UUID) (*v3.Operation, error)
	GetInstance(ctx context.Context, id v3.UUID) (*v3.Instance, error)
	GetInstancePool(ctx context.Context, id v3.UUID) (*v3.InstancePool, error)
	GetInstanceType(ctx context.Context, id v3.UUID) (*v3.InstanceType, error)
	GetLoadBalancer(ctx context.Context, id v3.UUID) (*v3.LoadBalancer, error)
	ListInstances(ctx context.Context, opts ...v3.ListInstancesOpt) (*v3.ListInstancesResponse, error)
	ListLoadBalancers(ctx context.Context) (*v3.ListLoadBalancersResponse, error)
	ListSKSClusters(ctx context.Context) (*v3.ListSKSClustersResponse, error)
	UpdateLoadBalancer(ctx context.Context, id v3.UUID, req v3.UpdateLoadBalancerRequest) (*v3.Operation, error)
	UpdateLoadBalancerService(ctx context.Context, id v3.UUID, serviceID v3.UUID, req v3.UpdateLoadBalancerServiceRequest) (*v3.Operation, error)
	Wait(ctx context.Context, op *v3.Operation, states ...v3.OperationState) (*v3.Operation, error)
}

type apiClient struct {
	exo *v3.Client
}

type switchZone func(ctx context.Context, client *v3.Client, zone v3.ZoneName) (*v3.Client, error)

var switchZoneCallback switchZone = func(ctx context.Context, client *v3.Client, zone v3.ZoneName) (*v3.Client, error) {
	if zone != "" {
		zoneEndpoint, err := client.GetZoneAPIEndpoint(ctx, zone)
		if err != nil {
			return nil, err
		}

		client = client.WithEndpoint(zoneEndpoint)
	}

	if err := validateCredentials(ctx, client); err != nil {
		return nil, err
	}

	return client, nil
}

// validateCredentials returns an error if the Exoscale API rejects the client credentials.
// Listing zones doesn't require authentication, so switching zone doesn't validate them:
// a cheap authenticated call does. Only an authentication failure is reported, so that
// valid credentials missing the IAM permission for this call are still accepted.
func validateCredentials(ctx context.Context, client *v3.Client) error {
	_, err := client.ListLoadBalancers(ctx)
	if err == nil {
		return nil
	}

	var apiErr *v3.APIError
	if errors.Is(err, v3.ErrUnauthorized) ||
		(errors.Is(err, v3.ErrForbidden) && errors.As(err, &apiErr) &&
			strings.Contains(strings.ToLower(apiErr.Message), "invalid key")) {
		return fmt.Errorf("invalid Exoscale API credentials: %w", err)
	}

	infof("warning: unable to validate Exoscale API credentials: %v", err)

	return nil
}

func newAPIClient(ctx context.Context, config *globalConfig, zone v3.ZoneName, zoneCallback switchZone) (*apiClient, error) {
	creds, err := newAPICredentials(ctx, config)
	if err != nil {
		return nil, err
	}

	opts := []v3.ClientOpt{
		v3.ClientOptWithUserAgent(fmt.Sprintf("Exoscale-K8s-Cloud-Controller/%s", versionString)),
	}
	if config.APIEndpoint != "" {
		opts = append(opts, v3.ClientOptWithEndpoint(v3.Endpoint(config.APIEndpoint)))
	}

	exo, err := v3.NewClient(creds, opts...)
	if err != nil {
		return nil, err
	}

	exo, err = zoneCallback(ctx, exo, zone)
	if err != nil {
		return nil, err
	}

	return &apiClient{exo: exo}, nil
}

// newAPICredentials returns the credentials set in config. Those of a credentials file are re-read
// until ctx is done: the client uses a rotated API key without being restarted.
func newAPICredentials(ctx context.Context, config *globalConfig) (*credentials.Credentials, error) {
	switch {
	case config.APIKey != "" && config.APISecret != "":
		infof("using Exoscale actual API credentials (key + secret)")

		return credentials.NewStaticCredentials(config.APIKey, config.APISecret), nil

	case config.APICredentialsFile != "":
		infof("reading Exoscale API credentials from file %q (reloaded when it changes)", config.APICredentialsFile)

		creds, err := credentials.NewReloadableCredentials(
			ctx,
			credentials.JSONFileSource(config.APICredentialsFile),
			credentials.ReloadableOptWithLogHandler(logr.ToSlogHandler(klog.Background())),
		)
		if err != nil {
			return nil, fmt.Errorf("unable to read Exoscale API credentials file: %w", err)
		}

		return creds, nil

	default:
		return nil, errors.New("incomplete or missing Exoscale API credentials")
	}
}

func (c *apiClient) Wait(ctx context.Context, op *v3.Operation, states ...v3.OperationState) (*v3.Operation, error) {
	return c.exo.Wait(ctx, op, states...)
}
