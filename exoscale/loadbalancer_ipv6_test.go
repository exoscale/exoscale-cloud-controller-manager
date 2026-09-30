package exoscale

import (
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	v3 "github.com/exoscale/egoscale/v3"
)

var (
	testNLBIDv6        v3.UUID = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testNLBIPv6address         = "2a04:c43:e00:82a1:500:1:0:1"
)

// dualStackTestService returns a dual-stack Service exposing port 80 (NodePort 32672) and
// 443 (NodePort 32673) over IPv4, and 8080/8443 over IPv6.
func dualStackTestService(uid string, annotations map[string]string) *v1.Service {
	a := map[string]string{
		annotationLoadBalancerID:                    testNLBID.String(),
		annotationLoadBalancerName:                  testNLBName,
		annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
		annotationLoadBalancerIPAddressType:         ipAddressTypeDualStack,
		annotationLoadBalancerIPv6TargetPorts:       "80:8080,443:8443",
	}
	for k, v := range annotations {
		a[k] = v
	}

	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test",
			Namespace:   metav1.NamespaceDefault,
			UID:         types.UID(uid),
			Annotations: a,
		},
		Spec: v1.ServiceSpec{
			Ports: []v1.ServicePort{
				{Protocol: v1.ProtocolTCP, Port: 80, NodePort: 32672},
				{Protocol: v1.ProtocolTCP, Port: 443, NodePort: 32673},
			},
		},
	}
}

func testNodes() []*v1.Node {
	return []*v1.Node{{
		ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
		Status: v1.NodeStatus{
			NodeInfo:  v1.NodeSystemInfo{SystemUUID: testInstanceID.String()},
			Addresses: []v1.NodeAddress{{Type: v1.NodeExternalIP, Address: "2a04:c43:e00:82a1::1"}},
		},
	}}
}

// mockNLBs makes the client mock serve the NLB instances provided, adding the NLB services created
// through AddServiceToLoadBalancer to them. It returns the AddServiceToLoadBalancer requests per NLB.
func (ts *exoscaleCCMTestSuite) mockNLBs(nlbs ...*v3.LoadBalancer) map[v3.UUID][]v3.AddServiceToLoadBalancerRequest {
	added := make(map[v3.UUID][]v3.AddServiceToLoadBalancerRequest)
	byID := make(map[v3.UUID]*v3.LoadBalancer)

	for _, nlb := range nlbs {
		byID[nlb.ID] = nlb
		ts.p.client.(*exoscaleClientMock).
			On("GetLoadBalancer", ts.p.ctx, nlb.ID).
			Return(nlb, nil)
	}

	ts.p.client.(*exoscaleClientMock).
		On("AddServiceToLoadBalancer", ts.p.ctx, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			id := args.Get(1).(v3.UUID)
			req := args.Get(2).(v3.AddServiceToLoadBalancerRequest)
			added[id] = append(added[id], req)
			byID[id].Services = append(byID[id].Services, v3.LoadBalancerService{
				ID:       v3.UUID(ts.randomID()),
				Name:     req.Name,
				Port:     req.Port,
				Protocol: v3.LoadBalancerServiceProtocol(req.Protocol),
			})
		}).
		Return(&v3.Operation{}, nil)

	return added
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_dualStack_create() {
	var (
		service    = dualStackTestService(ts.randomID(), nil)
		nlbCreated = false
		nlb        = &v3.LoadBalancer{ID: testNLBID, Name: testNLBName, IP: testNLBIPaddressP}
		nlbIPv6    = &v3.LoadBalancer{
			ID:            testNLBIDv6,
			Name:          testNLBName + "-ipv6",
			IP:            net.ParseIP(testNLBIPv6address),
			Addressfamily: v3.LoadBalancerAddressfamilyInet6,
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstancePool", ts.p.ctx, testNLBServiceInstancePoolID).
		Return(&v3.InstancePool{
			ID:                 testNLBServiceInstancePoolID,
			PublicIPAssignment: v3.PublicIPAssignmentDual,
			Instances:          []v3.Instance{{ID: testInstanceID}},
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateLoadBalancer", ts.p.ctx, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbCreated = true
			ts.Require().Equal(v3.CreateLoadBalancerRequest{
				Addressfamily: v3.CreateLoadBalancerRequestAddressfamilyInet6,
				Name:          testNLBName + "-ipv6",
			}, args.Get(1))
		}).
		Return(&v3.Operation{Reference: &v3.OperationReference{ID: testNLBIDv6}}, nil)

	added := ts.mockNLBs(nlb, nlbIPv6)

	ts.p.kclient = fake.NewSimpleClientset(service)

	status, err := ts.p.loadBalancer.EnsureLoadBalancer(ts.p.ctx, "", service, testNodes())
	ts.Require().NoError(err)
	ts.Require().True(nlbCreated)
	ts.Require().Equal(&v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{
		{IP: testNLBIPaddress},
		{IP: testNLBIPv6address},
	}}, status)

	// The IPv6 NLB ID is persisted in the Service annotations.
	svc, err := ts.p.kclient.CoreV1().Services(service.Namespace).Get(ts.p.ctx, service.Name, metav1.GetOptions{})
	ts.Require().NoError(err)
	ts.Require().Equal(testNLBIDv6.String(), svc.Annotations[annotationLoadBalancerIPv6ID])

	// The IPv4 NLB services still target the NodePorts...
	ts.Require().Len(added[testNLBID], 2)
	for i, port := range []int64{32672, 32673} {
		ts.Require().Equal(port, added[testNLBID][i].TargetPort)
		ts.Require().Equal(port, added[testNLBID][i].Healthcheck.Port)
	}

	// ...while the IPv6 ones target the IPv6 target ports, on the same Instance Pool.
	ts.Require().Len(added[testNLBIDv6], 2)
	for i, port := range []int64{8080, 8443} {
		ts.Require().Equal(port, added[testNLBIDv6][i].TargetPort)
		ts.Require().Equal(port, added[testNLBIDv6][i].Healthcheck.Port)
		ts.Require().Equal(testNLBServiceInstancePoolID, added[testNLBIDv6][i].InstancePool.ID)
	}
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_dualStack_instancePoolNotDual() {
	service := dualStackTestService(ts.randomID(), nil)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstancePool", ts.p.ctx, testNLBServiceInstancePoolID).
		Return(&v3.InstancePool{ID: testNLBServiceInstancePoolID}, nil)

	ts.p.kclient = fake.NewSimpleClientset(service)

	_, err := ts.p.loadBalancer.EnsureLoadBalancer(ts.p.ctx, "", service, testNodes())
	ts.Require().ErrorContains(err, "is not dual-stack")
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "CreateLoadBalancer", mock.Anything, mock.Anything)
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "AddServiceToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)

	// The deprecated ipv6-enabled flag is honored.
	ipv6Enabled := true
	ts.SetupTest()
	ts.p.client.(*exoscaleClientMock).
		On("GetInstancePool", ts.p.ctx, testNLBServiceInstancePoolID).
		Return(&v3.InstancePool{ID: testNLBServiceInstancePoolID, Ipv6Enabled: &ipv6Enabled}, nil)
	ts.Require().NoError(ts.p.loadBalancer.(*loadBalancer).checkInstancePoolDualStack(ts.p.ctx, service, testNodes()))
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_dualStack_addressFamilyConflict() {
	// An external NLB referenced as IPv6 NLB, but actually IPv4 (e.g. shared with another Service).
	service := dualStackTestService(ts.randomID(), map[string]string{
		annotationLoadBalancerExternal: "true",
		annotationLoadBalancerIPv6ID:   testNLBIDv6.String(),
	})

	ts.p.client.(*exoscaleClientMock).
		On("GetInstancePool", ts.p.ctx, testNLBServiceInstancePoolID).
		Return(&v3.InstancePool{PublicIPAssignment: v3.PublicIPAssignmentDual}, nil)

	ts.mockNLBs(
		&v3.LoadBalancer{ID: testNLBID, Name: testNLBName, IP: testNLBIPaddressP},
		&v3.LoadBalancer{ID: testNLBIDv6, Name: "shared", IP: testNLBIPaddressP},
	)

	ts.p.kclient = fake.NewSimpleClientset(service)

	_, err := ts.p.loadBalancer.EnsureLoadBalancer(ts.p.ctx, "", service, testNodes())
	ts.Require().ErrorContains(err, `has address family "inet4", expected "inet6"`)
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "AddServiceToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "UpdateLoadBalancer", mock.Anything, mock.Anything, mock.Anything)

	// The same applies to an IPv6 NLB referenced as the IPv4 one.
	ts.SetupTest()
	service = dualStackTestService(ts.randomID(), map[string]string{annotationLoadBalancerIPAddressType: ipAddressTypeIPv4})
	ts.mockNLBs(&v3.LoadBalancer{ID: testNLBID, Name: testNLBName, Addressfamily: v3.LoadBalancerAddressfamilyInet6})
	ts.p.kclient = fake.NewSimpleClientset(service)

	_, err = ts.p.loadBalancer.EnsureLoadBalancer(ts.p.ctx, "", service, testNodes())
	ts.Require().ErrorContains(err, `has address family "inet6", expected "inet4"`)
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "AddServiceToLoadBalancer", mock.Anything, mock.Anything, mock.Anything)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_dualStack_downgrade() {
	var (
		nlbIPv6Deleted        = false
		nlbIPv6ServiceDeleted = false
		nlbIPv6               = &v3.LoadBalancer{
			ID:            testNLBIDv6,
			Name:          testNLBName + "-ipv6",
			IP:            net.ParseIP(testNLBIPv6address),
			Addressfamily: v3.LoadBalancerAddressfamilyInet6,
			Services: []v3.LoadBalancerService{
				{ID: testNLBServiceID, Port: 80, Protocol: v3.LoadBalancerServiceProtocolTCP},
			},
		}
	)

	// The ip-address-type annotation is back to its default (IPv4), but the IPv6 NLB is still referenced.
	service := dualStackTestService(ts.randomID(), map[string]string{annotationLoadBalancerIPv6ID: testNLBIDv6.String()})
	delete(service.Annotations, annotationLoadBalancerIPAddressType)

	ts.mockNLBs(&v3.LoadBalancer{ID: testNLBID, Name: testNLBName, IP: testNLBIPaddressP}, nlbIPv6)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteLoadBalancerService", ts.p.ctx, testNLBIDv6, testNLBServiceID).
		Run(func(_ mock.Arguments) { nlbIPv6ServiceDeleted = true }).
		Return(&v3.Operation{}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteLoadBalancer", ts.p.ctx, testNLBIDv6).
		Run(func(_ mock.Arguments) { nlbIPv6Deleted = true }).
		Return(&v3.Operation{}, nil)

	ts.p.kclient = fake.NewSimpleClientset(service)

	status, err := ts.p.loadBalancer.EnsureLoadBalancer(ts.p.ctx, "", service, testNodes())
	ts.Require().NoError(err)
	ts.Require().True(nlbIPv6ServiceDeleted)
	ts.Require().True(nlbIPv6Deleted)
	ts.Require().Equal(&v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}}}, status)
	ts.p.client.(*exoscaleClientMock).AssertNotCalled(ts.T(), "GetInstancePool", mock.Anything, mock.Anything)

	svc, err := ts.p.kclient.CoreV1().Services(service.Namespace).Get(ts.p.ctx, service.Name, metav1.GetOptions{})
	ts.Require().NoError(err)
	ts.Require().NotContains(svc.Annotations, annotationLoadBalancerIPv6ID)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancerDeleted_dualStack() {
	var (
		service = dualStackTestService(ts.randomID(), map[string]string{annotationLoadBalancerIPv6ID: testNLBIDv6.String()})
		deleted = make(map[v3.UUID]bool)
	)

	for _, id := range []v3.UUID{testNLBID, testNLBIDv6} {
		ts.p.client.(*exoscaleClientMock).
			On("GetLoadBalancer", ts.p.ctx, id).
			Return(&v3.LoadBalancer{
				ID: id,
				Services: []v3.LoadBalancerService{
					{ID: v3.UUID(ts.randomID()), Port: 80, Protocol: v3.LoadBalancerServiceProtocolTCP},
					{ID: v3.UUID(ts.randomID()), Port: 443, Protocol: v3.LoadBalancerServiceProtocolTCP},
				},
			}, nil)
	}

	ts.p.client.(*exoscaleClientMock).
		On("DeleteLoadBalancerService", ts.p.ctx, mock.Anything, mock.Anything).
		Return(&v3.Operation{}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteLoadBalancer", ts.p.ctx, mock.Anything).
		Run(func(args mock.Arguments) { deleted[args.Get(1).(v3.UUID)] = true }).
		Return(&v3.Operation{}, nil)

	ts.Require().NoError(ts.p.loadBalancer.EnsureLoadBalancerDeleted(ts.p.ctx, "", service))
	ts.Require().Equal(map[v3.UUID]bool{testNLBID: true, testNLBIDv6: true}, deleted)
	ts.p.client.(*exoscaleClientMock).AssertNumberOfCalls(ts.T(), "DeleteLoadBalancerService", 4)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_GetLoadBalancer_dualStack() {
	service := dualStackTestService(ts.randomID(), map[string]string{annotationLoadBalancerIPv6ID: testNLBIDv6.String()})

	ts.mockNLBs(
		&v3.LoadBalancer{ID: testNLBID, IP: testNLBIPaddressP},
		&v3.LoadBalancer{ID: testNLBIDv6, IP: net.ParseIP(testNLBIPv6address)},
	)

	status, exists, err := ts.p.loadBalancer.GetLoadBalancer(ts.p.ctx, "", service)
	ts.Require().NoError(err)
	ts.Require().True(exists)
	ts.Require().Equal(&v1.LoadBalancerStatus{Ingress: []v1.LoadBalancerIngress{
		{IP: testNLBIPaddress},
		{IP: testNLBIPv6address},
	}}, status)
}

func Test_getIPAddressType(t *testing.T) {
	tests := []struct {
		value   string
		want    string
		wantErr string
	}{
		{value: "", want: ipAddressTypeIPv4},
		{value: "ipv4", want: ipAddressTypeIPv4},
		{value: "DualStack", want: ipAddressTypeDualStack},
		{value: "ipv6", wantErr: "not supported yet"},
		{value: "both", wantErr: "invalid"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%q", tt.value), func(t *testing.T) {
			service := &v1.Service{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
			if tt.value != "" {
				service.Annotations[annotationLoadBalancerIPAddressType] = tt.value
			}

			got, err := getIPAddressType(service)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_parseIPv6TargetPorts(t *testing.T) {
	onePort := []v1.ServicePort{{Port: 80}}
	twoPorts := []v1.ServicePort{{Port: 80}, {Port: 443}}

	tests := []struct {
		name    string
		value   string
		ports   []v1.ServicePort
		want    map[int32]int64
		wantErr string
	}{
		{name: "missing", ports: onePort, wantErr: "is required"},
		{name: "single port", value: "8080", ports: onePort, want: map[int32]int64{80: 8080}},
		{name: "single port with several Service ports", value: "8080", ports: twoPorts, wantErr: "format"},
		{name: "mapping", value: "80:8080, 443:8443", ports: twoPorts, want: map[int32]int64{80: 8080, 443: 8443}},
		{name: "unmapped Service port", value: "80:8080", ports: twoPorts, wantErr: "no target port for Service port 443"},
		{name: "unknown Service port", value: "80:8080,8443:8443", ports: twoPorts, wantErr: "doesn't expose"},
		{name: "duplicate", value: "80:8080,80:8081", ports: onePort, wantErr: "more than once"},
		{name: "malformed entry", value: "80:8080,443", ports: twoPorts, wantErr: "invalid entry"},
		{name: "out of range", value: "80:70000", ports: onePort, wantErr: "invalid port"},
		{name: "not a number", value: "http", ports: onePort, wantErr: "invalid port"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &v1.Service{
				ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}},
				Spec:       v1.ServiceSpec{Ports: tt.ports},
			}
			if tt.value != "" {
				service.Annotations[annotationLoadBalancerIPv6TargetPorts] = tt.value
			}

			got, err := parseIPv6TargetPorts(service)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func Test_buildIPv6LoadBalancerFromAnnotations(t *testing.T) {
	uid := new(exoscaleCCMTestSuite).randomID()

	t.Run("defaults", func(t *testing.T) {
		service := dualStackTestService(uid, map[string]string{annotationLoadBalancerIPv6ID: testNLBIDv6.String()})

		lb, err := buildIPv6LoadBalancerFromAnnotations(service)
		require.NoError(t, err)
		require.Equal(t, testNLBIDv6, lb.ID)
		require.Equal(t, testNLBName+"-ipv6", lb.Name)
		require.Len(t, lb.Services, 2)
		for i, port := range []int64{8080, 8443} {
			require.Equal(t, fmt.Sprintf("%s-%d", uid, service.Spec.Ports[i].Port), lb.Services[i].Name)
			require.Equal(t, port, lb.Services[i].TargetPort)
			require.Equal(t, port, lb.Services[i].Healthcheck.Port)
		}

		// The IPv4 NLB spec is left untouched.
		lbIPv4, err := buildLoadBalancerFromAnnotations(service)
		require.NoError(t, err)
		require.Equal(t, int64(32672), lbIPv4.Services[0].TargetPort)
		require.Equal(t, int64(32672), lbIPv4.Services[0].Healthcheck.Port)
	})

	t.Run("overrides", func(t *testing.T) {
		service := dualStackTestService(uid, map[string]string{
			annotationLoadBalancerIPv6Name:            "custom",
			annotationLoadBalancerIPv6HealthCheckPort: "9000",
		})

		lb, err := buildIPv6LoadBalancerFromAnnotations(service)
		require.NoError(t, err)
		require.Equal(t, "custom", lb.Name)
		require.Equal(t, int64(8080), lb.Services[0].TargetPort)
		require.Equal(t, int64(9000), lb.Services[0].Healthcheck.Port)
		require.Equal(t, int64(9000), lb.Services[1].Healthcheck.Port)
	})

	t.Run("invalid healthcheck port", func(t *testing.T) {
		service := dualStackTestService(uid, map[string]string{annotationLoadBalancerIPv6HealthCheckPort: "0"})

		_, err := buildIPv6LoadBalancerFromAnnotations(service)
		require.ErrorContains(t, err, "invalid IPv6 healthcheck port")
	})
}

func Test_checkAddressFamily(t *testing.T) {
	require.NoError(t, checkAddressFamily(&v3.LoadBalancer{}, v3.LoadBalancerAddressfamilyInet4))
	require.NoError(t, checkAddressFamily(
		&v3.LoadBalancer{Addressfamily: v3.LoadBalancerAddressfamilyInet6},
		v3.LoadBalancerAddressfamilyInet6,
	))
	require.Error(t, checkAddressFamily(&v3.LoadBalancer{}, v3.LoadBalancerAddressfamilyInet6))
	require.Error(t, checkAddressFamily(
		&v3.LoadBalancer{Addressfamily: v3.LoadBalancerAddressfamilyInet6},
		v3.LoadBalancerAddressfamilyInet4,
	))
}
