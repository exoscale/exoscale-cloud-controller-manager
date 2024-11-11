package exoscale

import (
	"fmt"
	"net"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	v3 "github.com/exoscale/egoscale/v3"
)

var (
	testInstanceID             v3.UUID               = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testInstanceName                                 = new(exoscaleCCMTestSuite).randomString(10)
	testInstancePublicIPv4                           = "1.2.3.4"
	testInstancePrivateIPv4                          = "10.0.0.1"
	testInstancePublicIPv6                           = "fd00::123:4"
	testInstancePublicIPv4P                          = net.ParseIP(testInstancePublicIPv4)
	testInstancePublicIPv6P                          = net.ParseIP(testInstancePublicIPv6)
	testInstanceTypeAuthorized                       = true
	testInstanceTypeCPUs       int64                 = 2
	testInstanceTypeFamily     v3.InstanceTypeFamily = "standard"
	testInstanceTypeID         v3.UUID               = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testInstanceTypeMemory     int64                 = 4294967296
	testInstanceTypeSize       v3.InstanceTypeSize   = "medium"
)

func (ts *exoscaleCCMTestSuite) TestNodeAddresses() {
	resp := &v3.Instance{
		ID:       testInstanceID,
		Name:     testInstanceName,
		PublicIP: testInstancePublicIPv4P,
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	for _, tt := range []struct {
		name     string
		egoscale v3.Instance
		expected []v1.NodeAddress
	}{
		{
			name: "PublicIPv4",
			egoscale: v3.Instance{
				ID:       testInstanceID,
				Name:     testInstanceName,
				PublicIP: testInstancePublicIPv4P,
			},
			expected: []v1.NodeAddress{
				{
					Type:    v1.NodeHostName,
					Address: testInstanceName,
				},
				{
					Type:    v1.NodeExternalIP,
					Address: testInstancePublicIPv4,
				},
				{
					Type:    v1.NodeInternalIP,
					Address: testInstancePublicIPv4,
				},
			},
		},
		{
			name: "PublicIPv6",
			egoscale: v3.Instance{
				ID:          testInstanceID,
				Name:        testInstanceName,
				Ipv6Address: testInstancePublicIPv6P.String(),
			},
			expected: []v1.NodeAddress{
				{
					Type:    v1.NodeHostName,
					Address: testInstanceName,
				},
				{
					Type:    v1.NodeExternalIP,
					Address: testInstancePublicIPv6,
				},
				{
					Type:    v1.NodeInternalIP,
					Address: testInstancePublicIPv6,
				},
			},
		},
		{
			name: "DualStack",
			egoscale: v3.Instance{
				ID:          testInstanceID,
				Name:        testInstanceName,
				PublicIP:    testInstancePublicIPv4P,
				Ipv6Address: testInstancePublicIPv6P.String(),
			},
			expected: []v1.NodeAddress{
				{
					Type:    v1.NodeHostName,
					Address: testInstanceName,
				},
				{
					Type:    v1.NodeExternalIP,
					Address: testInstancePublicIPv4,
				},
				{
					Type:    v1.NodeInternalIP,
					Address: testInstancePublicIPv4,
				},
				{
					Type:    v1.NodeExternalIP,
					Address: testInstancePublicIPv6,
				},
				{
					Type:    v1.NodeInternalIP,
					Address: testInstancePublicIPv6,
				},
			},
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
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:       testInstanceID,
				Name:     testInstanceName,
				PublicIP: testInstancePublicIPv4P,
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	expected := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: testInstanceName,
		},
		{
			Type:    v1.NodeExternalIP,
			Address: testInstancePublicIPv4,
		},
		{
			Type:    v1.NodeInternalIP,
			Address: testInstancePublicIPv4,
		},
	}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID_WithIPV6Enabled() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:          testInstanceID,
				Name:        testInstanceName,
				PublicIP:    testInstancePublicIPv4P,
				Ipv6Address: testInstancePublicIPv6P.String(),
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	expected := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: testInstanceName,
		},
		{
			Type:    v1.NodeExternalIP,
			Address: testInstancePublicIPv4,
		},
		{
			Type:    v1.NodeInternalIP,
			Address: testInstancePublicIPv4,
		},
		{
			Type:    v1.NodeExternalIP,
			Address: testInstancePublicIPv6,
		},
		{
			Type:    v1.NodeInternalIP,
			Address: testInstancePublicIPv6,
		},
	}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID_WithPrivateNetworkIDs() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:       testInstanceID,
				Name:     testInstanceName,
				PublicIP: testInstancePublicIPv4P,
				PrivateNetworks: []v3.InstancePrivateNetworks{{
					ID: v3.UUID(new(exoscaleCCMTestSuite).randomID()),
				}},
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
			Annotations: map[string]string{
				cloudproviderapi.AnnotationAlphaProvidedIPAddr: testInstancePrivateIPv4,
			},
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	expected := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: testInstanceName,
		},
		{
			Type:    v1.NodeInternalIP,
			Address: testInstancePrivateIPv4,
		},
		{
			Type:    v1.NodeExternalIP,
			Address: testInstancePublicIPv4,
		},
	}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestNodeAddressesByProviderID_WithOnlyPrivateNetworkIDs() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:   v3.UUID(testInstanceID),
				Name: testInstanceName,
				PrivateNetworks: []v3.InstancePrivateNetworks{{
					ID: v3.UUID(new(exoscaleCCMTestSuite).randomID()),
				}},
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
			Annotations: map[string]string{
				cloudproviderapi.AnnotationAlphaProvidedIPAddr: testInstancePrivateIPv4,
			},
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	expected := []v1.NodeAddress{
		{
			Type:    v1.NodeHostName,
			Address: testInstanceName,
		},
		{
			Type:    v1.NodeInternalIP,
			Address: testInstancePrivateIPv4,
		},
	}

	actual, err := ts.p.instances.NodeAddressesByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	actual, err := ts.p.instances.InstanceID(ts.p.ctx, types.NodeName(testInstanceName))
	ts.Require().NoError(err)
	ts.Require().Equal(testInstanceID.String(), actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceType() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID: testInstanceID,
				InstanceType: &v3.InstanceType{
					ID: testInstanceTypeID,
				},
				Name: testInstanceName,
			},
			nil,
		)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstanceType", ts.p.ctx, testInstanceTypeID).
		Return(
			&v3.InstanceType{
				Authorized: &testInstanceTypeAuthorized,
				Cpus:       testInstanceTypeCPUs,
				Family:     testInstanceTypeFamily,
				ID:         testInstanceTypeID,
				Memory:     testInstanceTypeMemory,
				Size:       testInstanceTypeSize,
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
				SystemUUID: testInstanceID.String(),
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
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID: testInstanceID,
				InstanceType: &v3.InstanceType{
					ID: testInstanceTypeID,
				},
				Name: testInstanceName,
			},
			nil,
		)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstanceType", ts.p.ctx, testInstanceTypeID).
		Return(
			&v3.InstanceType{
				Authorized: &testInstanceTypeAuthorized,
				Cpus:       testInstanceTypeCPUs,
				Family:     testInstanceTypeFamily,
				ID:         testInstanceTypeID,
				Memory:     testInstanceTypeMemory,
				Size:       testInstanceTypeSize,
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})
	testInstanceTypeName := getInstanceTypeName(testInstanceTypeFamily, testInstanceTypeSize)
	fmt.Println(testInstanceTypeName)
	actual, err := ts.p.instances.InstanceTypeByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	actual, err := ts.p.instances.CurrentNodeName(ts.p.ctx, testInstanceName)
	ts.Require().NoError(err)
	ts.Require().Equal(types.NodeName(testInstanceName), actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceExistsByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:   testInstanceID,
				Name: testInstanceName,
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	exists, err := ts.p.instances.InstanceExistsByProviderID(ts.p.ctx, providerPrefix+testInstanceID.String())
	ts.Require().NoError(err)
	ts.Require().True(exists)

	// // Test with non-existent instance:

	// nonExistentID := ts.randomID()

	// ts.p.client.(*exoscaleClientMock).
	// 	On("GetInstance", ts.p.ctx, nonExistentID).
	// 	Return(&v3.Instance{}, v3.ErrNotFound)

	// exists, err = ts.p.instances.InstanceExistsByProviderID(ts.p.ctx, providerPrefix+nonExistentID)
	// ts.Require().NoError(err)
	// ts.Require().False(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdownByProviderID() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:    testInstanceID,
				Name:  testInstanceName,
				State: v3.InstanceStateStopped,
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
				SystemUUID: testInstanceID.String(),
			},
		},
	})

	shutdown, err := ts.p.instances.InstanceShutdownByProviderID(
		ts.p.ctx,
		providerPrefix+testInstanceID.String(),
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
