package exoscale

import (
	"fmt"

	"github.com/exoscale/egoscale"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const (
	testInstanceID              = "8a3a817d-3874-477c-adaf-2b2ce9172528"
	testInstanceProviderID      = "exoscale://" + testInstanceID
	testInstanceName            = "k8s-master"
	testInstanceIP              = "159.100.251.253"
	testInstanceServiceOffering = "Medium"
	testInstanceZoneName        = "ch-zrh-1"
)

func newMockInstanceAPINotFound() (*cloudProvider, *testServer) {
	ts := newTestServer(testHTTPResponse{200, `
{"listvirtualmachinesresponse": {}}`})

	return &cloudProvider{
		client: egoscale.NewClient(ts.URL, "KEY", "SECRET"),
	}, ts
}

func newMockInstanceAPI() (*cloudProvider, *testServer) {
	fakenode := &v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID,
			},
		},
	}
	clientset := fake.NewSimpleClientset(fakenode)

	ts := newTestServer(testHTTPResponse{200, fmt.Sprintf(`
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
			"zonename": "%s"
		}
	]
}}`, testInstanceID, testInstanceName, testInstanceIP, testInstanceServiceOffering, testInstanceZoneName)})

	return &cloudProvider{
		client:  egoscale.NewClient(ts.URL, "KEY", "SECRET"),
		kclient: clientset,
	}, ts
}
