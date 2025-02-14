package exoscale

import (
	"context"

	egoscale "github.com/exoscale/egoscale/v2"

	"github.com/stretchr/testify/mock"
)

type exoscaleClientMock struct {
	mock.Mock
}

func (m *exoscaleClientMock) CreateNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) (*egoscale.NetworkLoadBalancer, error) {
	args := m.Called(ctx, zone, nlb)
	return args.Get(0).(*egoscale.NetworkLoadBalancer), args.Error(1)
}

func (m *exoscaleClientMock) CreateNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService,
) (*egoscale.NetworkLoadBalancerService, error) {
	args := m.Called(ctx, zone, nlb, svc)
	return args.Get(0).(*egoscale.NetworkLoadBalancerService), args.Error(1)
}

func (m *exoscaleClientMock) DeleteNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) error {
	args := m.Called(ctx, zone, nlb)
	return args.Error(0)
}

func (m *exoscaleClientMock) DeleteNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService,
) error {
	args := m.Called(ctx, zone, nlb, svc)
	return args.Error(0)
}

func (m *exoscaleClientMock) GetInstance(ctx context.Context, zone, id string) (*egoscale.Instance, error) {
	args := m.Called(ctx, zone, id)
	return args.Get(0).(*egoscale.Instance), args.Error(1)
}

func (m *exoscaleClientMock) GetInstanceType(ctx context.Context, zone, id string) (*egoscale.InstanceType, error) {
	args := m.Called(ctx, zone, id)
	return args.Get(0).(*egoscale.InstanceType), args.Error(1)
}

func (m *exoscaleClientMock) GetNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	id string,
) (*egoscale.NetworkLoadBalancer, error) {
	args := m.Called(ctx, zone, id)
	return args.Get(0).(*egoscale.NetworkLoadBalancer), args.Error(1)
}

func (m *exoscaleClientMock) ListInstances(
	ctx context.Context,
	zone string,
	opts ...egoscale.ListInstancesOpt,
) ([]*egoscale.Instance, error) {
	args := m.Called(ctx, zone, opts)
	return args.Get(0).([]*egoscale.Instance), args.Error(1)
}

func (m *exoscaleClientMock) ListNetworkLoadBalancers(
	ctx context.Context,
	zone string,
) ([]*egoscale.NetworkLoadBalancer, error) {
	args := m.Called(ctx, zone)
	return args.Get(0).([]*egoscale.NetworkLoadBalancer), args.Error(1)
}

func (m *exoscaleClientMock) ListSKSClusters(ctx context.Context, zone string) ([]*egoscale.SKSCluster, error) {
	args := m.Called(ctx, zone)
	return args.Get(0).([]*egoscale.SKSCluster), args.Error(1)
}

func (m *exoscaleClientMock) UpdateNetworkLoadBalancer(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
) error {
	args := m.Called(ctx, zone, nlb)
	return args.Error(0)
}

func (m *exoscaleClientMock) UpdateNetworkLoadBalancerService(
	ctx context.Context,
	zone string,
	nlb *egoscale.NetworkLoadBalancer,
	svc *egoscale.NetworkLoadBalancerService,
) error {
	args := m.Called(ctx, zone, nlb, svc)
	return args.Error(0)
}
