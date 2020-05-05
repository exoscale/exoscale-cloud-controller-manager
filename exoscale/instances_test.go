package exoscale

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestNodeAddresses(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeAddress, err := instance.NodeAddresses(ctx, types.NodeName(testInstanceName))

	require.NoError(t, err)
	require.NotNil(t, nodeAddress)

	expectedAddresses := []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: testInstanceIP},
	}

	require.Equal(t, expectedAddresses, nodeAddress)
}

func TestNodeAddressesByProviderID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeAddress, err := instance.NodeAddressesByProviderID(ctx, testInstanceProviderID)

	require.NoError(t, err)
	require.NotNil(t, nodeAddress)

	expectedAddresses := []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: testInstanceIP},
	}

	require.Equal(t, expectedAddresses, nodeAddress)
}

func TestInstanceID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	node, err := instance.InstanceID(ctx, types.NodeName(testInstanceName))

	require.NoError(t, err)

	require.Equal(t, node, testInstanceID)
}

func TestInstanceType(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeType, err := instance.InstanceType(ctx, types.NodeName(testInstanceName))

	require.NoError(t, err)

	require.Equal(t, nodeType, testInstanceServiceOffering)
}

func TestInstanceTypeByProviderID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeType, err := instance.InstanceTypeByProviderID(ctx, testInstanceProviderID)

	require.NoError(t, err)

	require.Equal(t, nodeType, testInstanceServiceOffering)
}

func TestCurrentNodeName(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeName, err := instance.CurrentNodeName(ctx, testInstanceName)

	require.NoError(t, err)
	require.NotNil(t, nodeName)

	require.Equal(t, nodeName, types.NodeName(testInstanceName))
}

func TestInstanceExistsByProviderID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}

	nodeExist, err := instance.InstanceExistsByProviderID(ctx, testInstanceProviderID)

	require.NoError(t, err)
	require.True(t, nodeExist)

	ts.Close()

	p, ts = newMockInstanceAPINotFound()
	instance = &instances{p: p}
	defer ts.Close()

	nodeExist, err = instance.InstanceExistsByProviderID(ctx, "exoscale://00113bd2-d6cc-418e-831d-2d4785f6e5b6")

	require.NoError(t, err)
	require.False(t, nodeExist)
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	ctx := context.Background()
	p, ts := newMockInstanceAPI()
	instance := &instances{p: p}
	defer ts.Close()

	nodeShutdown, err := instance.InstanceShutdownByProviderID(ctx, testInstanceProviderID)

	require.NoError(t, err)
	require.False(t, nodeShutdown)
}
