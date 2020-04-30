package exoscale

import (
	"context"

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

func nodeAddresses(vm *egoscale.VirtualMachine) ([]v1.NodeAddress, error) {
	var addresses []v1.NodeAddress
	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeHostName, Address: vm.HostName})

	nic := vm.DefaultNic()

	addresses = append(addresses, v1.NodeAddress{Type: v1.NodeExternalIP, Address: nic.IPAddress.String()})

	return addresses, nil
}
