package exoscale

import (
	"context"
	"strings"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func (c *cloudProvider) virtualMachineByName(ctx context.Context, name types.NodeName) (*egoscale.VirtualMachine, error) {
	r, err := c.client.GetWithContext(ctx, egoscale.VirtualMachine{Name: string(name)})
	if err != nil {
		return nil, err
	}

	return r.(*egoscale.VirtualMachine), nil
}

func (c *cloudProvider) virtualMachineByProviderID(ctx context.Context, providerID string) (*egoscale.VirtualMachine, error) {
	id := formatProviderID(providerID)
	r, err := c.client.GetWithContext(ctx, egoscale.VirtualMachine{ID: egoscale.MustParseUUID(id)})
	if err != nil {
		return nil, err
	}

	return r.(*egoscale.VirtualMachine), nil
}

func nodeAddresses(vm *egoscale.VirtualMachine) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: vm.Name})

	nic := vm.DefaultNic()

	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: nic.IPAddress.String()})

	return addresses, nil
}

func formatProviderID(providerID string) string {
	return strings.TrimLeft(providerID, "exoscale://")
}
