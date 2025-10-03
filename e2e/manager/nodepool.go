package manager

import (
	"context"
	"fmt"
	"time"

	exoscale "github.com/exoscale/egoscale/v3"
)

type NodepoolManager struct {
	client     *exoscale.Client
	config     *Config
	clusterID  exoscale.UUID
	nodepoolID exoscale.UUID
	nodepool   *exoscale.SKSNodepool
}

func NewNodepoolManager(client *exoscale.Client, config *Config, clusterID string) (*NodepoolManager, error) {
	uuid, err := exoscale.ParseUUID(clusterID)
	if err != nil {
		return nil, fmt.Errorf("invalid cluster ID: %w", err)
	}

	return &NodepoolManager{
		client:    client,
		config:    config,
		clusterID: uuid,
	}, nil
}

func (nm *NodepoolManager) CreateNodepool(ctx context.Context, size int64) error {
	nodepoolName := fmt.Sprintf("nodepool-%s", nm.config.TestID)

	instanceTypes, err := nm.client.ListInstanceTypes(ctx)
	if err != nil {
		return fmt.Errorf("failed to list instance types: %w", err)
	}

	instanceType, err := instanceTypes.FindInstanceTypeByIdOrFamilyAndSize(nm.config.InstanceType)
	if err != nil {
		return fmt.Errorf("failed to find instance type %q: %w", nm.config.InstanceType, err)
	}

	createReq := exoscale.CreateSKSNodepoolRequest{
		Name:         nodepoolName,
		InstanceType: &instanceType,
		Size:         size,
		DiskSize:     nm.config.DiskSize,
	}

	ctx, cancel := context.WithTimeout(ctx, nm.config.Timeouts.NodepoolCreate)
	defer cancel()

	op, err := nm.client.CreateSKSNodepool(ctx, nm.clusterID, createReq)
	if err != nil {
		return fmt.Errorf("failed to create nodepool: %w", err)
	}

	_, err = nm.client.Wait(ctx, op, exoscale.OperationStateSuccess)
	if err != nil {
		return fmt.Errorf("failed to wait for nodepool creation: %w", err)
	}

	nm.nodepoolID = op.Reference.ID

	nodepool, err := nm.client.GetSKSNodepool(ctx, nm.clusterID, nm.nodepoolID)
	if err != nil {
		return fmt.Errorf("failed to get nodepool details: %w", err)
	}

	nm.nodepool = nodepool
	return nil
}

func (nm *NodepoolManager) ResizeNodepool(ctx context.Context, newSize int64) error {
	if nm.nodepoolID == "" {
		return fmt.Errorf("nodepool not created")
	}

	ctx, cancel := context.WithTimeout(ctx, nm.config.Timeouts.NodepoolResize)
	defer cancel()

	scaleReq := exoscale.ScaleSKSNodepoolRequest{
		Size: newSize,
	}

	_, err := nm.client.ScaleSKSNodepool(ctx, nm.clusterID, nm.nodepoolID, scaleReq)
	if err != nil {
		return fmt.Errorf("failed to scale nodepool: %w", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for nodepool to scale")
		case <-ticker.C:
			nodepool, err := nm.client.GetSKSNodepool(ctx, nm.clusterID, nm.nodepoolID)
			if err != nil {
				continue
			}

			if nodepool.Size == newSize && nodepool.State == exoscale.SKSNodepoolStateRunning {
				nm.nodepool = nodepool
				return nil
			}
		}
	}
}

func (nm *NodepoolManager) DeleteNodepool(ctx context.Context) error {
	if nm.nodepoolID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, nm.config.Timeouts.NodepoolDelete)
	defer cancel()

	_, err := nm.client.DeleteSKSNodepool(ctx, nm.clusterID, nm.nodepoolID)
	if err != nil {
		return fmt.Errorf("failed to delete nodepool: %w", err)
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for nodepool deletion")
		case <-ticker.C:
			_, err := nm.client.GetSKSNodepool(ctx, nm.clusterID, nm.nodepoolID)
			if err != nil {
				return nil
			}
		}
	}
}

func (nm *NodepoolManager) GetNodepoolID() string {
	return nm.nodepoolID.String()
}

func (nm *NodepoolManager) GetNodepool() *exoscale.SKSNodepool {
	return nm.nodepool
}

func (nm *NodepoolManager) GetInstancePoolSecurityGroups(ctx context.Context) ([]exoscale.SecurityGroup, error) {
	if nm.nodepool == nil || nm.nodepool.InstancePool == nil {
		return nil, fmt.Errorf("nodepool or instance pool not available")
	}

	instancePool, err := nm.client.GetInstancePool(ctx, nm.nodepool.InstancePool.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get instance pool: %w", err)
	}

	if len(instancePool.SecurityGroups) == 0 {
		return nil, fmt.Errorf("instance pool has no security groups")
	}

	var securityGroups []exoscale.SecurityGroup
	for _, sg := range instancePool.SecurityGroups {
		securityGroups = append(securityGroups, exoscale.SecurityGroup{ID: sg.ID})
	}

	return securityGroups, nil
}

func (nm *NodepoolManager) WaitForNodepoolRunning(ctx context.Context) error {
	timeout := time.After(nm.config.Timeouts.NodepoolCreate)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for nodepool to be running")
		case <-ticker.C:
			nodepool, err := nm.client.GetSKSNodepool(ctx, nm.clusterID, nm.nodepoolID)
			if err != nil {
				return fmt.Errorf("failed to get nodepool status: %w", err)
			}

			if nodepool.State == exoscale.SKSNodepoolStateRunning {
				nm.nodepool = nodepool
				return nil
			}
		}
	}
}
