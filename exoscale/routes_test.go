package exoscale

import (
	"net"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	cloudprovider "k8s.io/cloud-provider"

	v3 "github.com/exoscale/egoscale/v3"
	"github.com/stretchr/testify/mock"
)

var (
	testClusterName               = "test-cluster"
	testVPCID             v3.UUID = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testSubnetID          v3.UUID = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testPeerSubnetID      v3.UUID = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testInstanceVPCIPv4           = "172.16.0.10"
	testPodCIDR                   = "10.244.1.0/24"
	testRouteOwned                = routeDescriptionPrefix(testClusterName) + testInstanceName
	testRouteTarget               = routeTarget(testInstanceVPCIPv4)
	testRouteTarget_stale         = routeTarget("172.16.0.99")

	testConfig_routes = routesConfig{
		Enabled:           true,
		VPC:               testVPCID.String(),
		Subnet:            testSubnetID.String(),
		AdditionalSubnets: []string{testPeerSubnetID.String()},
	}
)

func (ts *exoscaleCCMTestSuite) createRouteTestNode() {
	_, err := ts.p.kclient.CoreV1().Nodes().Create(ts.p.ctx, &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
		Spec:       v1.NodeSpec{ProviderID: providerPrefix + testInstanceID.String()},
	}, metav1.CreateOptions{})
	ts.Require().NoError(err)
}

func (ts *exoscaleCCMTestSuite) mockSubnet(instances ...v3.SubnetInstances) *mock.Call {
	return ts.p.client.(*exoscaleClientMock).
		On("GetSubnet", ts.p.ctx, testVPCID, testSubnetID).
		Return(&v3.Subnet{ID: testSubnetID, Instances: instances}, nil)
}

func (ts *exoscaleCCMTestSuite) mockRoutes(subnet v3.UUID, routes ...v3.ListRouteEntry) {
	ts.p.client.(*exoscaleClientMock).
		On("ListRoutes", ts.p.ctx, testVPCID, subnet).
		Return(&v3.ListRoutesResponse{Routes: routes}, nil)
}

var testSubnetInstance = v3.SubnetInstances{ID: testInstanceID, Ipv4: net.ParseIP(testInstanceVPCIPv4)}

func (ts *exoscaleCCMTestSuite) TestListRoutes() {
	ts.createRouteTestNode()
	ts.mockSubnet(testSubnetInstance)
	ts.mockRoutes(testSubnetID,
		v3.ListRouteEntry{Destination: testPodCIDR, Target: testRouteTarget, Description: testRouteOwned},
		v3.ListRouteEntry{Destination: "10.244.2.0/24", Target: testRouteTarget_stale, Description: testRouteOwned},
		v3.ListRouteEntry{Destination: "10.244.3.0/24", Target: testRouteTarget, Description: testRouteOwned},
		v3.ListRouteEntry{Destination: "0.0.0.0/0", Target: testRouteTarget, Description: "not ours"},
	)
	ts.mockRoutes(testPeerSubnetID,
		v3.ListRouteEntry{Destination: testPodCIDR, Target: testRouteTarget, Description: testRouteOwned},
		v3.ListRouteEntry{Destination: "10.244.2.0/24", Target: testRouteTarget_stale, Description: testRouteOwned},
	)

	list, err := ts.p.routes.ListRoutes(ts.p.ctx, testClusterName)
	ts.Require().NoError(err)
	ts.Require().ElementsMatch([]*cloudprovider.Route{
		// complete in every Subnet
		{Name: testPodCIDR, TargetNode: types.NodeName(testInstanceName), DestinationCIDR: testPodCIDR},
		// target is no Node
		{Name: testRouteTarget_stale, DestinationCIDR: "10.244.2.0/24", Blackhole: true},
		// 10.244.3.0/24 is missing from the peer Subnet: omitted so that it gets re-created
	}, list)
}

func (ts *exoscaleCCMTestSuite) TestCreateRoute() {
	ts.createRouteTestNode()
	client := ts.p.client.(*exoscaleClientMock)
	op := &v3.Operation{ID: v3.UUID(ts.randomID())}

	client.
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(&v3.Instance{ID: testInstanceID, IPForwarding: new(true)}, nil)
	// not attached yet, then attached
	ts.mockSubnet().Once()
	ts.mockSubnet(testSubnetInstance).Once()
	client.
		On("AttachInstanceToSubnet", ts.p.ctx, testVPCID, testSubnetID, v3.AttachInstanceToSubnetRequest{
			Instance: &v3.InstanceRef{ID: testInstanceID},
		}).
		Return(op, nil)
	client.
		On("Wait", ts.p.ctx, op, []v3.OperationState{v3.OperationStateSuccess}).
		Return(op, nil)

	staleID := v3.UUID(ts.randomID())
	ts.mockRoutes(testSubnetID)
	ts.mockRoutes(testPeerSubnetID,
		v3.ListRouteEntry{ID: staleID, Destination: testPodCIDR, Target: testRouteTarget_stale, Description: testRouteOwned},
	)
	client.On("DeleteRoute", ts.p.ctx, testVPCID, testPeerSubnetID, staleID).Return(nil)

	req := v3.CreateRouteRequest{Description: testRouteOwned, Destination: testPodCIDR, Target: testRouteTarget}
	client.On("CreateRoute", ts.p.ctx, testVPCID, testSubnetID, req).Return(&v3.Route{}, nil)
	client.On("CreateRoute", ts.p.ctx, testVPCID, testPeerSubnetID, req).Return(&v3.Route{}, nil)

	err := ts.p.routes.CreateRoute(ts.p.ctx, testClusterName, "", &cloudprovider.Route{
		TargetNode:      types.NodeName(testInstanceName),
		DestinationCIDR: testPodCIDR,
	})
	ts.Require().NoError(err)
	client.AssertExpectations(ts.T())
}

func (ts *exoscaleCCMTestSuite) TestCreateRoute_noIPForwarding() {
	ts.createRouteTestNode()
	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, testInstanceID).
		Return(&v3.Instance{ID: testInstanceID}, nil)

	err := ts.p.routes.CreateRoute(ts.p.ctx, testClusterName, "", &cloudprovider.Route{
		TargetNode:      types.NodeName(testInstanceName),
		DestinationCIDR: testPodCIDR,
	})
	ts.Require().ErrorContains(err, "ip-forwarding")
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "CreateRoute")
}

func (ts *exoscaleCCMTestSuite) TestDeleteRoute_blackhole() {
	client := ts.p.client.(*exoscaleClientMock)
	staleID := v3.UUID(ts.randomID())
	ts.mockRoutes(testSubnetID,
		v3.ListRouteEntry{ID: v3.UUID(ts.randomID()), Destination: testPodCIDR, Target: testRouteTarget, Description: testRouteOwned},
		v3.ListRouteEntry{ID: staleID, Destination: testPodCIDR, Target: testRouteTarget_stale, Description: testRouteOwned},
	)
	ts.mockRoutes(testPeerSubnetID)
	client.On("DeleteRoute", ts.p.ctx, testVPCID, testSubnetID, staleID).Return(nil)

	err := ts.p.routes.DeleteRoute(ts.p.ctx, testClusterName, &cloudprovider.Route{
		Name:            testRouteTarget_stale,
		DestinationCIDR: testPodCIDR,
		Blackhole:       true,
	})
	ts.Require().NoError(err)
	client.AssertNumberOfCalls(ts.T(), "DeleteRoute", 1)
}
