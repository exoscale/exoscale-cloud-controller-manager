package exoscale

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	egoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"
)

// Label value must be '(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])?')
// Invalid characters will be removed
var labelInvalidCharsRegex = regexp.MustCompile(`^[^A-Za-z0-9]|[^A-Za-z0-9]$|([^-A-Za-z0-9_.])`)

type instances struct {
	p   *cloudProvider
	cfg *instancesConfig
}

func newInstances(provider *cloudProvider, config *instancesConfig) cloudprovider.Instances {
	return &instances{
		p:   provider,
		cfg: config,
	}
}

// NodeAddresses returns the addresses of the specified instance.
func (i *instances) NodeAddresses(ctx context.Context, nodeName types.NodeName) ([]v1.NodeAddress, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverride(nodeName)
	if override != nil {
		if n := len(override.Addresses); n > 0 {
			nodeAddresses := make([]v1.NodeAddress, n)
			for i, a := range override.Addresses {
				nodeAddresses[i] = v1.NodeAddress{
					Type:    v1.NodeAddressType(a.Type),
					Address: a.Address,
				}
			}
			return nodeAddresses, nil
		} else if override.External {
			return []v1.NodeAddress{}, nil // returning no address makes the stock node-controller skip address re-assignment
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return nil, fmt.Errorf("no override found (Exoscale API disabled)")
	}

	id, err := i.InstanceID(ctx, nodeName)
	if err != nil {
		return nil, err
	}

	return i.NodeAddressesByProviderID(ctx, id)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverrideByProviderID(providerID)
	if override != nil {
		if n := len(override.Addresses); n > 0 {
			nodeAddresses := make([]v1.NodeAddress, n)
			for i, a := range override.Addresses {
				nodeAddresses[i] = v1.NodeAddress{
					Type:    v1.NodeAddressType(a.Type),
					Address: a.Address,
				}
			}
			return nodeAddresses, nil
		} else if override.External {
			return []v1.NodeAddress{}, nil // returning no address makes the stock node-controller skip address re-assignment
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return nil, fmt.Errorf("no override found (Exoscale API disabled)")
	}

	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	addresses := []v1.NodeAddress{}

	if instance.PublicIPAddress != nil {
		addresses = append(
			addresses,
			v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.PublicIPAddress.String()},
		)
	}

	if i.p.client != nil && instance.PrivateNetworkIDs != nil && len(*instance.PrivateNetworkIDs) > 0 {
		if node, _ := i.p.kclient.CoreV1().Nodes().Get(ctx, *instance.Name, metav1.GetOptions{}); node != nil {
			if providedIP, ok := node.ObjectMeta.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
				addresses = append(
					addresses,
					v1.NodeAddress{Type: v1.NodeInternalIP, Address: providedIP},
				)
			}
		}
	}

	if instance.IPv6Enabled != nil && *instance.IPv6Enabled {
		addresses = append(
			addresses,
			v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.IPv6Address.String()},
		)
	}

	return addresses, nil
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
// ADDENDUM:
// InstanceID is used internally to build the ProviderID used in ...ByProviderID methods
// (see GetInstanceProviderID in https://github.com/kubernetes/cloud-provider/blob/master/cloud.go)
// TL;DR: ProviderID = "exoscale://<InstanceID>"
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverride(nodeName)
	if override != nil {
		if override.External {
			if override.ExternalID != "" {
				return override.ExternalID, nil
			} else {
				return defaultInstanceOverrideExternalID(override.Name), nil
			}
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return "", fmt.Errorf("no override found (Exoscale API disabled)")
	}

	node, err := i.p.kclient.CoreV1().Nodes().Get(ctx, string(nodeName), metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to retrieve node %s from the apiserver: %s", nodeName, err)
	}

	// K8s SystemUUID = Exoscale InstanceID
	return node.Status.NodeInfo.SystemUUID, nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(ctx context.Context, nodeName types.NodeName) (string, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverride(nodeName)
	if override != nil {
		if override.Type != "" {
			return override.Type, nil
		} else if override.External {
			return "external", nil
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return "", fmt.Errorf("no override found (Exoscale API disabled)")
	}

	id, err := i.InstanceID(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return i.InstanceTypeByProviderID(ctx, id)
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverrideByProviderID(providerID)
	if override != nil {
		if override.Type != "" {
			return override.Type, nil
		} else if override.External {
			return "external", nil
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return "", fmt.Errorf("no override found (Exoscale API disabled)")
	}

	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return "", err
	}

	instanceType, err := i.p.client.GetInstanceType(ctx, i.p.zone, *instance.InstanceTypeID)
	if err != nil {
		return "", err
	}

	return labelInvalidCharsRegex.ReplaceAllString(getInstanceTypeName(*instanceType.Family, *instanceType.Size), ""), nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(_ context.Context, _ string, _ []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(_ context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverrideByProviderID(providerID)
	if override != nil {
		if override.External {
			return true, nil
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return false, nil
	}

	_, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		if errors.Is(err, exoapi.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	// first look for a statically-configured override
	override := i.cfg.getInstanceOverrideByProviderID(providerID)
	if override != nil {
		if override.External {
			return false, cloudprovider.NotImplemented
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return false, fmt.Errorf("no override found (Exoscale API disabled)")
	}

	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return *instance.State == "stopping" || *instance.State == "stopped", nil
}

func (c *refreshableExoscaleClient) GetInstance(ctx context.Context, zone, id string) (*egoscale.Instance, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetInstance(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		id,
	)
}

func (c *refreshableExoscaleClient) GetInstanceType(ctx context.Context, zone, id string) (*egoscale.InstanceType, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.GetInstanceType(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		id)
}

func (c *refreshableExoscaleClient) ListInstances(
	ctx context.Context,
	zone string,
	opts ...egoscale.ListInstancesOpt,
) ([]*egoscale.Instance, error) {
	c.RLock()
	defer c.RUnlock()

	return c.exo.ListInstances(
		exoapi.WithEndpoint(ctx, exoapi.NewReqEndpoint(c.apiEnvironment, zone)),
		zone,
		opts...,
	)
}

// Instance Type name is <family>.<size>
func getInstanceTypeName(family string, size string) string {
	return family + "." + size
}
