package exoscale

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *cloudProvider) computeInstanceByName(ctx context.Context, name types.NodeName) (*egoscale.VirtualMachine, error) {
	r, err := c.client.GetWithContext(ctx, egoscale.VirtualMachine{Name: string(name)})
	if err != nil {
		return nil, err
	}

	return r.(*egoscale.VirtualMachine), nil
}

func (c *cloudProvider) computeInstanceByProviderID(ctx context.Context, providerID string) (*egoscale.VirtualMachine, error) {
	id, err := formatProviderID(providerID)
	if err != nil {
		return nil, err
	}

	uuid, err := egoscale.ParseUUID(id)
	if err != nil {
		return nil, err
	}

	r, err := c.client.GetWithContext(ctx, egoscale.VirtualMachine{ID: uuid})
	if err != nil {
		return nil, err
	}

	return r.(*egoscale.VirtualMachine), nil
}

func nodeAddresses(instance *egoscale.VirtualMachine) ([]v1.NodeAddress, error) {
	nic := instance.DefaultNic()
	if nic == nil {
		return nil, fmt.Errorf("default NIC not found for instance %q", instance.ID.String())
	}

	return []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: nic.IPAddress.String()},
	}, nil
}

func formatProviderID(providerID string) (string, error) {
	if providerID == "" {
		return "", errors.New("provider ID cannot be empty")
	}

	const prefix = "exoscale://"

	if !strings.HasPrefix(providerID, prefix) {
		return "", fmt.Errorf("provider ID %q is missing prefix %q", providerID, prefix)
	}

	return strings.TrimPrefix(providerID, prefix), nil
}
