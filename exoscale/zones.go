package exoscale

import (
	"context"
	"fmt"

	"github.com/exoscale/egoscale/v3/metadata"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

type zones struct {
	p *cloudProvider
}

func newZones(provider *cloudProvider) cloudprovider.Zones {
	return &zones{
		p: provider,
	}
}

// GetZone returns the Zone containing the current failure zone and locality region that the program is running in
// In most cases, this method is called from the kubelet querying a local metadata service to acquire its zone.
// For the case of external cloud providers, use GetZoneByProviderID or GetZoneByNodeName since GetZone
// can no longer be called from the kubelets.
func (z zones) GetZone(ctx context.Context) (cloudprovider.Zone, error) {
	zone, err := metadata.Get(ctx, metadata.AvailabilityZone)
	if err != nil {
		return cloudprovider.Zone{}, err
	}

	return cloudprovider.Zone{Region: zone}, nil
}

// GetZoneByProviderID returns the Zone containing the current zone and locality region of the node specified by
// providerID. This method is particularly used in the context of external cloud providers where node initialization
// must be done outside the kubelets.
func (z *zones) GetZoneByProviderID(_ context.Context, providerID string) (cloudprovider.Zone, error) {
	// first look for a statically-configured override
	override := z.p.cfg.Instances.getInstanceOverrideByProviderID(providerID)
	if override != nil {
		if override.External {
			if override.Region != "" {
				return cloudprovider.Zone{Region: override.Region}, nil
			} else {
				return cloudprovider.Zone{Region: "external"}, nil
			}
		}
	}

	// Use Exoscale API ?
	if z.p.cfg.Instances.ExternalOnly {
		return cloudprovider.Zone{}, fmt.Errorf("no instance override found (Exoscale API disabled)")
	}

	// The Exoscale is set at Cloud Controller Manager level, cluster Nodes cannot be in a different zone.
	return cloudprovider.Zone{Region: z.p.zone}, nil
}

// GetZoneByNodeName returns the Zone containing the current zone and locality region of the node specified by node
// name. This method is particularly used in the context of external cloud providers where node initialization must
// be done outside the kubelets.
func (z *zones) GetZoneByNodeName(ctx context.Context, nodeName types.NodeName) (cloudprovider.Zone, error) {
	// first look for a statically-configured override
	override := z.p.cfg.Instances.getInstanceOverride(nodeName)
	if override != nil {
		if override.External {
			if override.Region != "" {
				return cloudprovider.Zone{Region: override.Region}, nil
			} else {
				return cloudprovider.Zone{Region: "external"}, nil
			}
		}
	}

	// Use Exoscale API ?
	if z.p.cfg.Instances.ExternalOnly {
		return cloudprovider.Zone{}, fmt.Errorf("no instance override found (Exoscale API disabled)")
	}

	node, err := z.p.kclient.CoreV1().Nodes().Get(ctx, string(nodeName), metav1.GetOptions{})
	if err != nil {
		return cloudprovider.Zone{}, fmt.Errorf(
			"failed to retrieve node %s from the apiserver: %s",
			nodeName,
			err,
		)
	}

	return z.GetZoneByProviderID(ctx, node.Status.NodeInfo.SystemUUID)
}
