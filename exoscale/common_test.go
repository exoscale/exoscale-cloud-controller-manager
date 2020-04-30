package exoscale

import (
	"github.com/exoscale/egoscale"
)

func newFakeInstanceAPI() (*cloudProvider, *testServer) {
	ts := newServer(response{200, jsonContentType, `
{"listvirtualmachinesresponse": {
	"count": 1,
	"virtualmachine": [
		{
			"displayname": "k8s-master",
			"hypervisor": "KVM",
			"id": "8a3a817d-3874-477c-adaf-2b2ce9172528",
			"keypair": "test",
			"name": "k8s-master",
			"nic": [
			  {
				"broadcasturi": "vlan://untagged",
				"gateway": "159.100.248.1",
				"id": "1bd61d54-580b-4808-9534-4b6ef2b9dab4",
				"ipaddress": "159.100.251.253",
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
			"serviceofferingname": "Medium",
			"state": "Running",
			"templateid": "2dc5d673-46df-4151-9b91-bc966f5b819b",
			"templatename": "Linux Ubuntu 18.04 LTS 64-bit",
			"zoneid": "381d0a95-ed4a-4ad9-b41c-b97073c1a433",
			"zonename": "ch-dk-2"
		}
	]
}}`})

	return &cloudProvider{
		client: egoscale.NewClient(ts.URL, "KEY", "SECRET"),
	}, ts
}
