package exoscale

import (
	"context"

	v3 "github.com/exoscale/egoscale/v3"

	"github.com/stretchr/testify/mock"
)

type exoscaleClientMock struct {
	mock.Mock
}

func (m *exoscaleClientMock) CreateLoadBalancer(
	ctx context.Context,
	req v3.CreateLoadBalancerRequest,
) (*v3.Operation, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) AddServiceToLoadBalancer(
	ctx context.Context,
	id v3.UUID,
	req v3.AddServiceToLoadBalancerRequest,
) (*v3.Operation, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) DeleteLoadBalancer(
	ctx context.Context,
	id v3.UUID,
) (*v3.Operation, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) DeleteLoadBalancerService(
	ctx context.Context,
	id v3.UUID,
	serviceID v3.UUID,
) (*v3.Operation, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) GetInstance(ctx context.Context, id v3.UUID) (*v3.Instance, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*v3.Instance), args.Error(1)
}

func (m *exoscaleClientMock) GetInstanceType(ctx context.Context, id v3.UUID) (*v3.InstanceType, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*v3.InstanceType), args.Error(1)
}

func (m *exoscaleClientMock) GetLoadBalancer(ctx context.Context, id v3.UUID) (*v3.LoadBalancer, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*v3.LoadBalancer), args.Error(1)
}

func (m *exoscaleClientMock) ListInstances(
	ctx context.Context,
	opts ...v3.ListInstancesOpt,
) (*v3.ListInstancesResponse, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(*v3.ListInstancesResponse), args.Error(1)
}

func (m *exoscaleClientMock) UpdateLoadBalancer(
	ctx context.Context,
	id v3.UUID,
	req v3.UpdateLoadBalancerRequest,
) (*v3.Operation, error) {
	args := m.Called(ctx, id, req)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) UpdateLoadBalancerService(
	ctx context.Context,
	id v3.UUID,
	serviceID v3.UUID,
	req v3.UpdateLoadBalancerServiceRequest,
) (*v3.Operation, error) {
	args := m.Called(ctx, id, serviceID, req)
	return args.Get(0).(*v3.Operation), args.Error(1)
}

func (m *exoscaleClientMock) Wait(
	ctx context.Context,
	op *v3.Operation,
	states ...v3.OperationState,
) (*v3.Operation, error) {
	args := m.Called(ctx, op, states)
	return args.Get(0).(*v3.Operation), args.Error(1)
}
