package exoscale

import (
	"net"

	egoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
)

var (
	testInstanceID                     = new(exoscaleCCMTestSuite).randomID()
	testInstanceName                   = new(exoscaleCCMTestSuite).randomString(10)
	testInstancePublicIPAddress        = "1.2.3.4"
	testInstancePublicIPAddressP       = net.ParseIP(testInstancePublicIPAddress)
	testInstanceTypeAuthorized         = true
	testInstanceTypeCPUs         int64 = 2
	testInstanceTypeFamily             = "standard"
	testInstanceTypeID                 = new(exoscaleCCMTestSuite).randomID()
	testInstanceTypeMemory       int64 = 4294967296
	testInstanceTypeSize               = "medium"
)

func (ts *exoscaleCCMTestSuite) TestNodeAddresses() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:              &testInstanceID,
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPAddressP,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	expected := []v1.NodeAddress{{
		Type:    v1.NodeExternalIP,
		Address: testInstancePublicIPAddress,
	}}

	actual, err := ts.p.instances.NodeAddresses(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:              &testInstanceID,
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPAddressP,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	expected := []v1.NodeAddress{{
		Type:    v1.NodeExternalIP,
		Address: testInstancePublicIPAddress,
	}}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceID() {
	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	actual, err := ts.p.instances.InstanceID(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceID)
}

func (ts *exoscaleCCMTestSuite) TestInstanceType() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:             &testInstanceID,
				InstanceTypeID: &testInstanceTypeID,
				Name:           &testInstanceName,
			},
			nil,
		)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstanceType", ts.p.ctx, ts.p.zone, testInstanceTypeID).
		Return(
			&egoscale.InstanceType{
				Authorized: &testInstanceTypeAuthorized,
				CPUs:       &testInstanceTypeCPUs,
				Family:     &testInstanceTypeFamily,
				ID:         &testInstanceTypeID,
				Memory:     &testInstanceTypeMemory,
				Size:       &testInstanceTypeSize,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	actual, err := ts.p.instances.InstanceType(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceTypeSize)
}

func (ts *exoscaleCCMTestSuite) TestInstanceTypeByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:             &testInstanceID,
				InstanceTypeID: &testInstanceTypeID,
				Name:           &testInstanceName,
			},
			nil,
		)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstanceType", ts.p.ctx, ts.p.zone, testInstanceTypeID).
		Return(
			&egoscale.InstanceType{
				Authorized: &testInstanceTypeAuthorized,
				CPUs:       &testInstanceTypeCPUs,
				Family:     &testInstanceTypeFamily,
				ID:         &testInstanceTypeID,
				Memory:     &testInstanceTypeMemory,
				Size:       &testInstanceTypeSize,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	actual, err := ts.p.instances.InstanceTypeByProviderID(ts.p.ctx, providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().Equal(actual, testInstanceTypeSize)
}

func (ts *exoscaleCCMTestSuite) AddSSHKeyToAllInstances() {
	ts.T().Skip(cloudprovider.NotImplemented)
}

func (ts *exoscaleCCMTestSuite) TestCurrentNodeName() {
	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	actual, err := ts.p.instances.CurrentNodeName(ts.p.ctx, testInstanceName)
	ts.Require().NoError(err)
	ts.Require().Equal(actual, types.NodeName(testInstanceName))
}

func (ts *exoscaleCCMTestSuite) TestInstanceExistsByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:   &testInstanceID,
				Name: &testInstanceName,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	exists, err := ts.p.instances.InstanceExistsByProviderID(ts.p.ctx, providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().True(exists)

	// Test with non-existent instance:

	nonExistentID := ts.randomID()

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, nonExistentID).
		Return(new(egoscale.Instance), exoapi.ErrNotFound)

	exists, err = ts.p.instances.InstanceExistsByProviderID(ts.p.ctx, providerPrefix+nonExistentID)
	ts.Require().NoError(err)
	ts.Require().False(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdownByProviderID() {
	testInstanceStateShutdown := "stopped"

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:    &testInstanceID,
				Name:  &testInstanceName,
				State: &testInstanceStateShutdown,
			},
			nil,
		)

	ts.p.kclient = fake.NewSimpleClientset(&v1.Node{
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

	shutdown, err := ts.p.instances.InstanceShutdownByProviderID(
		ts.p.ctx,
		providerPrefix+testInstanceID,
	)
	ts.Require().NoError(err)
	ts.Require().True(shutdown)
}
