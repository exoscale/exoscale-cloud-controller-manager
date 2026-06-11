package exoscale

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	cloudprovider "k8s.io/cloud-provider"
	cloudproviderapi "k8s.io/cloud-provider/api"

	v3 "github.com/exoscale/egoscale/v3"
)

func (ts *exoscaleCCMTestSuite) testNode() *v1.Node {
	return &v1.Node{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: testInstanceName,
		},
		Spec: v1.NodeSpec{
			ProviderID: providerPrefix + testInstanceID.String(),
		},
		Status: v1.NodeStatus{
			NodeInfo: v1.NodeSystemInfo{
				SystemUUID: testInstanceID.String(),
			},
		},
	}
}

func (ts *exoscaleCCMTestSuite) TestInstanceExists() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:   testInstanceID,
				Name: testInstanceName,
			},
			nil,
		)

	exists, err := ts.p.instancesV2.InstanceExists(ts.p.ctx, ts.testNode())
	ts.Require().NoError(err)
	ts.Require().True(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceExists_uninitializedNode() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:   testInstanceID,
				Name: testInstanceName,
			},
			nil,
		)

	// before initialization by the CCM, the node has no spec.providerID:
	// the kubelet-reported system UUID is used instead
	node := ts.testNode()
	node.Spec.ProviderID = ""

	exists, err := ts.p.instancesV2.InstanceExists(ts.p.ctx, node)
	ts.Require().NoError(err)
	ts.Require().True(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdown() {
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

	shutdown, err := ts.p.instancesV2.InstanceShutdown(ts.p.ctx, ts.testNode())
	ts.Require().NoError(err)
	ts.Require().True(shutdown)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdown_running() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID:    testInstanceID,
				Name:  testInstanceName,
				State: v3.InstanceStateRunning,
			},
			nil,
		)

	shutdown, err := ts.p.instancesV2.InstanceShutdown(ts.p.ctx, ts.testNode())
	ts.Require().NoError(err)
	ts.Require().False(shutdown)
}

func (ts *exoscaleCCMTestSuite) TestInstanceMetadata() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID: testInstanceID,
				InstanceType: &v3.InstanceType{
					ID: testInstanceTypeID,
				},
				Name:     testInstanceName,
				PublicIP: testInstancePublicIPv4P,
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

	expected := &cloudprovider.InstanceMetadata{
		ProviderID:   providerPrefix + testInstanceID.String(),
		InstanceType: getInstanceTypeName(testInstanceTypeFamily, testInstanceTypeSize),
		NodeAddresses: []v1.NodeAddress{
			{Type: v1.NodeHostName, Address: testInstanceName},
			{Type: v1.NodeExternalIP, Address: testInstancePublicIPv4},
			{Type: v1.NodeInternalIP, Address: testInstancePublicIPv4},
		},
		Zone:   testZone,
		Region: testZone,
	}

	actual, err := ts.p.instancesV2.InstanceMetadata(ts.p.ctx, ts.testNode())
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}

func (ts *exoscaleCCMTestSuite) TestInstanceMetadata_withPrivateNetworkIDs() {
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(
			&v3.Instance{
				ID: testInstanceID,
				InstanceType: &v3.InstanceType{
					ID: testInstanceTypeID,
				},
				Name:     testInstanceName,
				PublicIP: testInstancePublicIPv4P,
				PrivateNetworks: []v3.InstancePrivateNetworks{{
					ID: v3.UUID(ts.randomID()),
				}},
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

	node := ts.testNode()
	node.ObjectMeta.Annotations = map[string]string{
		cloudproviderapi.AnnotationAlphaProvidedIPAddr: testInstancePrivateIPv4,
	}

	expected := []v1.NodeAddress{
		{Type: v1.NodeHostName, Address: testInstanceName},
		{Type: v1.NodeInternalIP, Address: testInstancePrivateIPv4},
		{Type: v1.NodeExternalIP, Address: testInstancePublicIPv4},
	}

	actual, err := ts.p.instancesV2.InstanceMetadata(ts.p.ctx, node)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual.NodeAddresses)
}

// Statically-configured overrides
func (ts *exoscaleCCMTestSuite) TestInstanceExists_overrideExternal() {
	node := ts.testNode()
	node.ObjectMeta.Name = testInstanceOverrideRegexpNodeName
	node.Spec.ProviderID = ""
	node.Status.NodeInfo.SystemUUID = ""

	exists, err := ts.p.instancesV2.InstanceExists(ts.p.ctx, node)
	ts.Require().NoError(err)
	ts.Require().True(exists)
}

func (ts *exoscaleCCMTestSuite) TestInstanceShutdown_overrideExternal() {
	node := ts.testNode()
	node.ObjectMeta.Name = testInstanceOverrideRegexpNodeName
	node.Spec.ProviderID = testInstanceOverrideRegexpProviderID
	node.Status.NodeInfo.SystemUUID = ""

	shutdown, err := ts.p.instancesV2.InstanceShutdown(ts.p.ctx, node)
	ts.Require().Equal(cloudprovider.NotImplemented, err)
	ts.Require().False(shutdown)
}

func (ts *exoscaleCCMTestSuite) TestInstanceMetadata_overrideExternal() {
	node := ts.testNode()
	node.ObjectMeta.Name = testInstanceOverrideRegexpNodeName
	node.Spec.ProviderID = ""
	node.Status.NodeInfo.SystemUUID = ""

	expected := &cloudprovider.InstanceMetadata{
		ProviderID:   testInstanceOverrideRegexpProviderID,
		InstanceType: testInstanceOverrideExternalType,
		NodeAddresses: []v1.NodeAddress{
			{Type: v1.NodeInternalIP, Address: testInstanceOverrideAddress_internal},
			{Type: v1.NodeExternalIP, Address: testInstanceOverrideAddress_external},
		},
		Zone:   testInstanceOverrideExternalRegion,
		Region: testInstanceOverrideExternalRegion,
	}

	actual, err := ts.p.instancesV2.InstanceMetadata(ts.p.ctx, node)
	ts.Require().NoError(err)
	ts.Require().Equal(expected, actual)
}
