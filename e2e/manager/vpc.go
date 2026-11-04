package manager

import (
	"context"
	"fmt"

	exoscale "github.com/exoscale/egoscale/v3"
)

type VPCManager struct {
	client  *exoscale.Client
	config  *Config
	vpcID   exoscale.UUID
	subnets []exoscale.UUID
}

func NewVPCManager(client *exoscale.Client, config *Config) *VPCManager {
	return &VPCManager{
		client: client,
		config: config,
	}
}

func (vm *VPCManager) CreateVPC(ctx context.Context) error {
	op, err := vm.client.CreateVpc(ctx, exoscale.CreateVpcRequest{
		Name: fmt.Sprintf("%s-vpc", vm.config.TestID),
	})
	if err != nil {
		return fmt.Errorf("failed to create VPC: %w", err)
	}

	if _, err = vm.client.Wait(ctx, op, exoscale.OperationStateSuccess); err != nil {
		return fmt.Errorf("failed to wait for VPC creation: %w", err)
	}

	vm.vpcID = op.Reference.ID
	return nil
}

// CreateSubnet creates a private IPv4 Subnet in the VPC and returns its ID.
func (vm *VPCManager) CreateSubnet(ctx context.Context, name, ipv4Block string) (exoscale.UUID, error) {
	op, err := vm.client.CreateSubnet(ctx, vm.vpcID, exoscale.CreateSubnetRequest{
		Name:          fmt.Sprintf("%s-%s", vm.config.TestID, name),
		AddressSpace:  exoscale.CreateSubnetRequestAddressSpacePrivate,
		Addressfamily: exoscale.CreateSubnetRequestAddressfamilyInet4,
		Ipv4Block:     ipv4Block,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create Subnet: %w", err)
	}

	if _, err = vm.client.Wait(ctx, op, exoscale.OperationStateSuccess); err != nil {
		return "", fmt.Errorf("failed to wait for Subnet creation: %w", err)
	}

	vm.subnets = append(vm.subnets, op.Reference.ID)
	return op.Reference.ID, nil
}

func (vm *VPCManager) GetSubnet(ctx context.Context, id exoscale.UUID) (*exoscale.Subnet, error) {
	return vm.client.GetSubnet(ctx, vm.vpcID, id)
}

func (vm *VPCManager) ListRoutes(ctx context.Context, subnet exoscale.UUID) ([]exoscale.ListRouteEntry, error) {
	res, err := vm.client.ListRoutes(ctx, vm.vpcID, subnet)
	if err != nil {
		return nil, fmt.Errorf("failed to list routes: %w", err)
	}
	return res.Routes, nil
}

// DeleteVPC deletes the Subnets with any route left behind, then the VPC: instances must be
// detached first.
func (vm *VPCManager) DeleteVPC(ctx context.Context) error {
	if vm.vpcID == "" {
		return nil
	}

	for _, subnet := range vm.subnets {
		routes, err := vm.ListRoutes(ctx, subnet)
		if err != nil {
			return err
		}
		for _, route := range routes {
			// implicit VPC routes go away with their Subnet
			if route.Kind != exoscale.ListRouteEntryKindSubnet {
				continue
			}
			if err := vm.client.DeleteRoute(ctx, vm.vpcID, subnet, route.ID); err != nil {
				return fmt.Errorf("failed to delete route %s: %w", route.ID, err)
			}
		}

		if err := vm.client.DeleteSubnet(ctx, vm.vpcID, subnet); err != nil {
			return fmt.Errorf("failed to delete Subnet %s: %w", subnet, err)
		}
	}

	if err := vm.client.DeleteVpc(ctx, vm.vpcID); err != nil {
		return fmt.Errorf("failed to delete VPC: %w", err)
	}

	return nil
}

func (vm *VPCManager) GetVPCID() string {
	return vm.vpcID.String()
}
