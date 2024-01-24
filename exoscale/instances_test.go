package exoscale

import (
	"fmt"
	"net"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"

	egoscale "github.com/exoscale/egoscale/v2"
	exoapi "github.com/exoscale/egoscale/v2/api"

	"k8s.io/utils/ptr"
)

var (
	testInstanceID                   = new(exoscaleCCMTestSuite).randomID()
	testInstanceName                 = new(exoscaleCCMTestSuite).randomString(10)
	testInstancePublicIPv4           = "1.2.3.4"
	testInstancePublicIPv6           = "fd00::123:4"
	testInstancePublicIPv4P          = net.ParseIP(testInstancePublicIPv4)
	testInstancePublicIPv6P          = net.ParseIP(testInstancePublicIPv6)
	testInstanceTypeAuthorized       = true
	testInstanceTypeCPUs       int64 = 2
	testInstanceTypeFamily           = "standard"
	testInstanceTypeID               = new(exoscaleCCMTestSuite).randomID()
	testInstanceTypeMemory     int64 = 4294967296
	testInstanceTypeSize             = "medium"
)

func (ts *exoscaleCCMTestSuite) TestNodeAddresses() {
	resp := &egoscale.Instance{
		ID:              &testInstanceID,
		Name:            &testInstanceName,
		PublicIPAddress: &testInstancePublicIPv4P,
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			resp,
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

	for _, tt := range []struct {
		name     string
		egoscale egoscale.Instance
		expected []v1.NodeAddress
	}{
		{
			name: "PublicIPv4",
			egoscale: egoscale.Instance{
				ID:              &testInstanceID,
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPv4P,
			},
			expected: []v1.NodeAddress{{
				Type:    v1.NodeHostName,
				Address: testInstanceName,
			}, {
				Type:    v1.NodeExternalIP,
				Address: testInstancePublicIPv4,
			}},
		},
		{
			name: "PublicIPv6",
			egoscale: egoscale.Instance{
				ID:          &testInstanceID,
				Name:        &testInstanceName,
				IPv6Address: &testInstancePublicIPv6P,
				IPv6Enabled: ptr.To(true),
			},
			expected: []v1.NodeAddress{{
				Type:    v1.NodeHostName,
				Address: testInstanceName,
			}, {
				Type:    v1.NodeExternalIP,
				Address: testInstancePublicIPv6,
			}},
		},
		{
			name: "DualStack",
			egoscale: egoscale.Instance{
				ID:              &testInstanceID,
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPv4P,
				IPv6Address:     &testInstancePublicIPv6P,
				IPv6Enabled:     ptr.To(true),
			},
			expected: []v1.NodeAddress{{
				Type:    v1.NodeHostName,
				Address: testInstanceName,
			}, {
				Type:    v1.NodeExternalIP,
				Address: testInstancePublicIPv4,
			}, {
				Type:    v1.NodeExternalIP,
				Address: testInstancePublicIPv6,
			}},
		},
	} {
		ts.Run(tt.name, func() {
			*resp = tt.egoscale

			actual, err := ts.p.instances.NodeAddresses(ts.p.ctx, types.NodeName(testInstanceName))
			ts.Require().NoError(err)
			ts.Require().Equal(tt.expected, actual)
		})
	}
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(
			&egoscale.Instance{
				ID:              &testInstanceID,
				Name:            &testInstanceName,
				PublicIPAddress: &testInstancePublicIPv4P,
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
		Type:    v1.NodeHostName,
		Address: testInstanceName,
	}, {
		Type:    v1.NodeExternalIP,
		Address: testInstancePublicIPv4,
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
	ts.Require().Equal(testInstanceID, actual)
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

	testInstanceTypeName := getInstanceTypeName(testInstanceTypeFamily, testInstanceTypeSize)
	actual, err := ts.p.instances.InstanceType(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceTypeName, actual)
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
	testInstanceTypeName := getInstanceTypeName(testInstanceTypeFamily, testInstanceTypeSize)
	fmt.Println(testInstanceTypeName)
	actual, err := ts.p.instances.InstanceTypeByProviderID(ts.p.ctx, providerPrefix+testInstanceID)
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceTypeName, actual)
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
	ts.Require().Equal(types.NodeName(testInstanceName), actual)
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

// Statically-configured overrides
func (ts *exoscaleCCMTestSuite) TestNodeAddresses_overrideExternal() {
	expected := []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: testInstanceOverrideAddress_internal},
		{Type: v1.NodeExternalIP, Address: testInstanceOverrideAddress_external},
	}

	actual, err := ts.p.instances.NodeAddresses(ts.p.ctx, types.NodeName(testInstanceOverrideRegexpNodeName))
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID_overrideExternal() {
	expected := []v1.NodeAddress{
		{Type: v1.NodeInternalIP, Address: testInstanceOverrideAddress_internal},
		{Type: v1.NodeExternalIP, Address: testInstanceOverrideAddress_external},
	}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, testInstanceOverrideRegexpProviderID)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceID_overrideExternal() {
	actual, err := ts.p.instances.InstanceID(ts.p.ctx, types.NodeName(testInstanceOverrideRegexpNodeName))
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceOverrideRegexpInstanceID, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceType_overrideExternal() {
	actual, err := ts.p.instances.InstanceType(ts.p.ctx, types.NodeName(testInstanceOverrideRegexpNodeName))
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceOverrideExternalType, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceTypeByProviderID_overrideExternal() {
	actual, err := ts.p.instances.InstanceTypeByProviderID(ts.p.ctx, testInstanceOverrideRegexpProviderID)
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceOverrideExternalType, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceExistsByProviderID_overrideExternal() {
	exists, err := ts.p.instances.InstanceExistsByProviderID(ts.p.ctx, testInstanceOverrideRegexpProviderID)
	ts.Require().NoError(err)
	ts.Require().True(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdownByProviderID_overrideExternal() {
	shutdown, err := ts.p.instances.InstanceShutdownByProviderID(ts.p.ctx, testInstanceOverrideRegexpProviderID)
	ts.Require().Equal(cloudprovider.NotImplemented, err)
	ts.Require().False(shutdown)
}
