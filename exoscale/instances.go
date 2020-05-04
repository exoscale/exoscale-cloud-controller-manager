package exoscale

import (
	"context"
	"regexp"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"
)

var labelInvalidCharsRegex *regexp.Regexp = regexp.MustCompile(`([^A-Za-z0-9][^-A-Za-z0-9_.]*)?[^A-Za-z0-9]`)

type instances struct {
	p *cloudProvider
}

func newInstances(provider *cloudProvider) cloudprovider.Instances {
	return &instances{
		p: provider,
	}
}

// NodeAddresses returns the addresses of the specified instance.
func (i *instances) NodeAddresses(ctx context.Context, name types.NodeName) ([]v1.NodeAddress, error) {
	instance, err := i.p.computeInstanceByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(instance)
}

// NodeAddressesByProviderID returns the addresses of the specified instance.
// The instance is specified using the providerID of the node. The
// ProviderID is a unique identifier of the node. This will not be called
// from the node whose nodeaddresses are being queried. i.e. local metadata
// services cannot be used in this method to obtain nodeaddresses
func (i *instances) NodeAddressesByProviderID(ctx context.Context, providerID string) ([]v1.NodeAddress, error) {
	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return nodeAddresses(instance)
}

// InstanceID returns the cloud provider ID of the node with the specified NodeName.
// Note that if the instance does not exist, we must return ("", cloudprovider.InstanceNotFound)
// cloudprovider.InstanceNotFound should NOT be returned for instances that exist but are stopped/sleeping
func (i *instances) InstanceID(ctx context.Context, nodeName types.NodeName) (string, error) {
	instance, err := i.p.computeInstanceByName(ctx, nodeName)
	if err != nil {
		return "", err
	}

	return instance.ID.String(), nil
}

// InstanceType returns the type of the specified instance.
func (i *instances) InstanceType(ctx context.Context, name types.NodeName) (string, error) {
	instance, err := i.p.computeInstanceByName(ctx, name)
	if err != nil {
		return "", err
	}

	return labelInvalidCharsRegex.ReplaceAllString(instance.ServiceOfferingName, ``), nil
}

// InstanceTypeByProviderID returns the type of the specified instance.
func (i *instances) InstanceTypeByProviderID(ctx context.Context, providerID string) (string, error) {
	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return "", err
	}

	return labelInvalidCharsRegex.ReplaceAllString(instance.ServiceOfferingName, ``), nil
}

// AddSSHKeyToAllInstances adds an SSH public key as a legal identity for all instances
// expected format for the key is standard ssh-keygen format: <protocol> <blob>
func (i *instances) AddSSHKeyToAllInstances(ctx context.Context, user string, keyData []byte) error {
	return cloudprovider.NotImplemented
}

// CurrentNodeName returns the name of the node we are currently running on
// On most clouds (e.g. GCE) this is the hostname, so we provide the hostname
func (i *instances) CurrentNodeName(ctx context.Context, hostname string) (types.NodeName, error) {
	return types.NodeName(hostname), nil
}

// InstanceExistsByProviderID returns true if the instance for the given provider exists.
// If false is returned with no error, the instance will be immediately deleted by the cloud controller manager.
// This method should still return true for instances that exist but are stopped/sleeping.
func (i *instances) InstanceExistsByProviderID(ctx context.Context, providerID string) (bool, error) {
	_, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return true, nil
}

// InstanceShutdownByProviderID returns true if the instance is shutdown in cloudprovider
func (i *instances) InstanceShutdownByProviderID(ctx context.Context, providerID string) (bool, error) {
	instance, err := i.p.computeInstanceByProviderID(ctx, providerID)
	if err != nil {
		return false, err
	}

	return egoscale.VirtualMachineState(instance.State) == egoscale.VirtualMachineShutdowned, nil
}
