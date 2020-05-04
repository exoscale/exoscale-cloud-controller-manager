package exoscale

import (
	"context"
	"fmt"
	"testing"

	"github.com/exoscale/egoscale"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	testInstanceID              = "8a3a817d-3874-477c-adaf-2b2ce9172528"
	testInstanceProviderID      = "exoscale://" + testInstanceID
	testInstanceName            = "k8s-master"
	testInstanceIP              = "159.100.251.253"
	testInstanceServiceOffering = "Medium"
)

func newMockInstanceAPI() (*instances, *testServer) {
	ts := newTestServer(testHTTPResponse{200, "application/json", fmt.Sprintf(`
{"listvirtualmachinesresponse": {
	"count": 1,
	"virtualmachine": [
		{
			"displayname": "k8s-master",
			"hypervisor": "KVM",
			"id": "%s",
			"keypair": "test",
			"name": "%s",
			"nic": [
			  {
				"broadcasturi": "vlan://untagged",
				"gateway": "159.100.248.1",
				"id": "1bd61d54-580b-4808-9534-4b6ef2b9dab4",
				"ipaddress": "%s",
				"isdefault": true,
				"macaddress": "00:70:30:00:00:00",
				"netmask": "255.255.252.0",
				"networkid": "d48bfccc-c11f-438f-8177-9cf6a40dc4f8",
				"networkname": "defaultGuestNetwork",
				"traffictype": "Guest",
				"type": "Shared"
			  }
			],
			"securitygroup": [
			  {
				"account": "exoscale",
				"id": "0f076a04-eb62-4201-b14e-e6c0e51cb60d",
				"name": "k8s-master"
			  }
			],
			"serviceofferingid": "b1191d3e-63aa-458b-ab00-0548748638c2",
			"serviceofferingname": "%s",
			"state": "Running",
			"templateid": "2dc5d673-46df-4151-9b91-bc966f5b819b",
			"templatename": "Linux Ubuntu 18.04 LTS 64-bit",
			"zoneid": "381d0a95-ed4a-4ad9-b41c-b97073c1a433",
			"zonename": "ch-dk-2"
		}
	]
}}`, testInstanceID, testInstanceName, testInstanceIP, testInstanceServiceOffering)})

	return &instances{
		&cloudProvider{
			client: egoscale.NewClient(ts.URL, "KEY", "SECRET"),
		},
	}, ts
}

func TestNodeAddresses(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeAddress, err := instances.NodeAddresses(ctx, types.NodeName(testInstanceName))

	require.Nil(t, err)
	require.NotNil(t, nodeAddress)

	expectedAddresses := []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: testInstanceIP},
	}

	require.Equal(t, expectedAddresses, nodeAddress)
}

func TestNodeAddressesByProviderID(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeAddress, err := instances.NodeAddressesByProviderID(ctx, testInstanceProviderID)

	require.Nil(t, err)
	require.NotNil(t, nodeAddress)

	expectedAddresses := []v1.NodeAddress{
		{Type: v1.NodeExternalIP, Address: testInstanceIP},
	}

	require.Equal(t, expectedAddresses, nodeAddress)
}

func TestInstanceID(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	node, err := instances.InstanceID(ctx, types.NodeName(testInstanceName))

	require.Nil(t, err)

	require.Equal(t, node, testInstanceID)
}

func TestInstanceType(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeType, err := instances.InstanceType(ctx, types.NodeName(testInstanceName))

	require.Nil(t, err)

	require.Equal(t, nodeType, testInstanceServiceOffering)
}

func TestInstanceTypeByProviderID(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeType, err := instances.InstanceTypeByProviderID(ctx, testInstanceProviderID)

	require.Nil(t, err)

	require.Equal(t, nodeType, testInstanceServiceOffering)
}

func TestCurrentNodeName(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeName, err := instances.CurrentNodeName(ctx, testInstanceName)

	require.Nil(t, err)
	require.NotNil(t, nodeName)

	require.Equal(t, nodeName, types.NodeName(testInstanceName))
}

func TestInstanceExistsByProviderID(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeExist, err := instances.InstanceExistsByProviderID(ctx, testInstanceProviderID)

	require.Nil(t, err)
	require.True(t, nodeExist)
}

func TestInstanceShutdownByProviderID(t *testing.T) {
	ctx := context.Background()
	instances, ts := newMockInstanceAPI()
	defer ts.Close()

	nodeShutdown, err := instances.InstanceShutdownByProviderID(ctx, testInstanceProviderID)

	require.Nil(t, err)
	require.False(t, nodeShutdown)
}
