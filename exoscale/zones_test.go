package exoscale

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
)

func (ts *exoscaleCCMTestSuite) TestGetZoneByProviderID() {
	expected := cloudprovider.Zone{Region: testZone}

	actual, err := ts.provider.zones.GetZoneByProviderID(context.Background(), "")
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestGetZoneByNodeName() {
	ts.provider.kclient = fake.NewSimpleClientset(&v1.Node{
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
	})

	expected := cloudprovider.Zone{Region: testZone}

	actual, err := ts.provider.zones.GetZoneByNodeName(context.Background(), types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}
