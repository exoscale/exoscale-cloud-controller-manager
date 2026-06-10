package exoscale

import (
	"context"
	"errors"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	v3 "github.com/exoscale/egoscale/v3"
)

type instancesV2 struct {
	p   *cloudProvider
	cfg *instancesConfig
}

func newInstancesV2(provider *cloudProvider, config *instancesConfig) cloudprovider.InstancesV2 {
	return &instancesV2{
		p:   provider,
		cfg: config,
	}
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instancesV2) InstanceExists(ctx context.Context, node *v1.Node) (bool, error) {
	// first look for a statically-configured override
	override := i.nodeInstanceOverride(node)
	if override != nil {
		if override.External {
			return true, nil
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return false, nil
	}

	_, err := i.computeInstanceByNode(ctx, node)
	if err != nil {
		if errors.Is(err, v3.ErrNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (i *instancesV2) InstanceShutdown(ctx context.Context, node *v1.Node) (bool, error) {
	// first look for a statically-configured override
	override := i.nodeInstanceOverride(node)
	if override != nil {
		if override.External {
			return false, cloudprovider.NotImplemented
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return false, fmt.Errorf("no override found (Exoscale API disabled)")
	}

	instance, err := i.computeInstanceByNode(ctx, node)
	if err != nil {
		return false, err
	}

	return instance.State == v3.InstanceStateStopping || instance.State == v3.InstanceStateStopped, nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields and labels in the Node object on registration.
func (i *instancesV2) InstanceMetadata(ctx context.Context, node *v1.Node) (*cloudprovider.InstanceMetadata, error) {
	meta := &cloudprovider.InstanceMetadata{}

	// first look for a statically-configured override
	override := i.nodeInstanceOverride(node)
	if override != nil {
		if override.Type != "" {
			meta.InstanceType = override.Type
		} else if override.External {
			meta.InstanceType = "external"
		}

		for _, a := range override.Addresses {
			meta.NodeAddresses = append(meta.NodeAddresses, v1.NodeAddress{
				Type:    v1.NodeAddressType(a.Type),
				Address: a.Address,
			})
		}

		if override.External {
			externalID := override.ExternalID
			if externalID == "" {
				externalID = defaultInstanceOverrideExternalID(override.Name)
			}
			meta.ProviderID = providerPrefix + externalID

			region := override.Region
			if region == "" {
				region = "external"
			}
			meta.Zone = region
			meta.Region = region

			return meta, nil
		}
	}

	// Use Exoscale API ?
	if i.cfg.ExternalOnly {
		return nil, fmt.Errorf("no override found (Exoscale API disabled)")
	}

	instance, err := i.computeInstanceByNode(ctx, node)
	if err != nil {
		return nil, err
	}

	meta.ProviderID = providerPrefix + instance.ID.String()

	// The Exoscale zone is set at Cloud Controller Manager level, cluster Nodes cannot be in a different zone.
	meta.Zone = i.p.zone
	meta.Region = i.p.zone

	if meta.InstanceType == "" {
		instanceType, err := i.p.client.GetInstanceType(ctx, instance.InstanceType.ID)
		if err != nil {
			return nil, err
		}

		meta.InstanceType = labelInvalidCharsRegex.ReplaceAllString(
			getInstanceTypeName(instanceType.Family, instanceType.Size),
			"",
		)
	}

	if len(meta.NodeAddresses) == 0 {
		meta.NodeAddresses = nodeAddressesFromInstance(node, instance)
	}

	return meta, nil
}

// nodeInstanceOverride returns the statically-configured override matching the
// node, looking up node.spec.providerID first, then the node name.
func (i *instancesV2) nodeInstanceOverride(node *v1.Node) *instancesOverrideConfig {
	if node.Spec.ProviderID != "" {
		if override := i.cfg.getInstanceOverrideByProviderID(node.Spec.ProviderID); override != nil {
			return override
		}
	}

	return i.cfg.getInstanceOverride(types.NodeName(node.Name))
}

// computeInstanceByNode returns the Exoscale Compute instance backing the node,
// from node.spec.providerID when set, otherwise from the kubelet-reported system
// UUID (K8s SystemUUID = Exoscale InstanceID) until the node is initialized.
func (i *instancesV2) computeInstanceByNode(ctx context.Context, node *v1.Node) (*v3.Instance, error) {
	providerID := node.Spec.ProviderID
	if providerID == "" {
		providerID = node.Status.NodeInfo.SystemUUID
	}

	return i.p.computeInstanceByProviderID(ctx, providerID)
}

// nodeAddressesFromInstance mirrors instances.NodeAddressesByProviderID, but reads
// the kubelet-provided IP annotation from the node object instead of querying the
// apiserver.
func nodeAddressesFromInstance(node *v1.Node, instance *v3.Instance) []v1.NodeAddress {
	addresses := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: instance.Name},
	}

	foundInternalIP := false
	if len(instance.PrivateNetworks) > 0 {
		if providedIP, ok := node.Annotations[cloudproviderapi.AnnotationAlphaProvidedIPAddr]; ok {
			addresses = append(
				addresses,
				v1.NodeAddress{Type: v1.NodeInternalIP, Address: providedIP},
			)
			foundInternalIP = true
		}
	}

	if instance.PublicIP != nil {
		addresses = append(
			addresses,
			v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.PublicIP.String()},
		)

		// if there is no internal IP, we use the public IP as internal IP
		// see spec here: https://kubernetes.io/docs/reference/node/node-status/#addresses
		if !foundInternalIP {
			addresses = append(
				addresses,
				v1.NodeAddress{Type: v1.NodeInternalIP, Address: instance.PublicIP.String()},
			)
		}
	}

	if instance.Ipv6Address != "" {
		addresses = append(
			addresses,
			v1.NodeAddress{Type: v1.NodeExternalIP, Address: instance.Ipv6Address},
		)

		// if there is no internal IP, we use the public IP as internal IP
		// see spec here: https://kubernetes.io/docs/reference/node/node-status/#addresses
		if !foundInternalIP {
			addresses = append(
				addresses,
				v1.NodeAddress{Type: v1.NodeInternalIP, Address: instance.Ipv6Address},
			)
		}
	}

	return addresses
}
