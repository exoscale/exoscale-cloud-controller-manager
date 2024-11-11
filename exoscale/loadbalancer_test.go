package exoscale

import (
	"fmt"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	v3 "github.com/exoscale/egoscale/v3"
)

var (
	testNLBCreatedAt                                                        = time.Now().UTC()
	testNLBDescription                                                      = new(exoscaleCCMTestSuite).randomString(10)
	testNLBID                         v3.UUID                               = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testNLBIPaddress                                                        = "1.2.3.4"
	testNLBIPaddressP                                                       = net.ParseIP(testNLBIPaddress)
	testNLBName                                                             = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceDescription                                               = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceHealthcheckInterval                                       = 10 * time.Second
	testNLBServiceHealthcheckMode     v3.LoadBalancerServiceHealthcheckMode = v3.LoadBalancerServiceHealthcheckModeHTTP
	testNLBServiceHealthcheckRetries  int64                                 = 2
	testNLBServiceHealthcheckTimeout                                        = 5 * time.Second
	testNLBServiceHealthcheckURI                                            = "/health"
	testNLBServiceID                  v3.UUID                               = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testNLBServiceInstancePoolID      v3.UUID                               = v3.UUID(new(exoscaleCCMTestSuite).randomID())
	testNLBServiceName                                                      = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceProtocol            v3.LoadBalancerServiceProtocol        = v3.LoadBalancerServiceProtocolTCP
	testNLBServiceStrategy            v3.LoadBalancerServiceStrategy        = v3.LoadBalancerServiceStrategyRoundRobin
)

func (ts *exoscaleCCMTestSuite) Test_newLoadBalancer() {
	actual := newLoadBalancer(ts.p, &testConfig_typical.LoadBalancer)
	ts.Require().Equal(&loadBalancer{p: ts.p, cfg: &testConfig_typical.LoadBalancer}, actual)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_isExternal() {
	type args struct {
		service *v1.Service
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "true",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationLoadBalancerExternal: "true",
						},
					},
				},
			},
			want: true,
		},
		{
			name: "false (explicit)",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							annotationLoadBalancerExternal: "false",
						},
					},
				},
			},
			want: false,
		},
		{
			name: "false (default)",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		ts.T().Run(tt.name, func(_ *testing.T) {
			if got := ts.p.loadBalancer.(*loadBalancer).isExternal(tt.args.service); got != tt.want {
				ts.T().Errorf("isExternal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_create() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		nlbCreated                    = false
		nlbServiceCreated             = false

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: metav1.NamespaceDefault,
				UID:       types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerDescription: testNLBDescription,
					annotationLoadBalancerName:        testNLBName,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol: v1.ProtocolTCP,
					Port:     int32(k8sServicePortPort),
					NodePort: int32(k8sServicePortNodePort),
				}},
			},
		}

		expectedNLB = &v3.LoadBalancer{
			Description: testNLBDescription,
			Name:        testNLBName,
			Services: []v3.LoadBalancerService{{
				Healthcheck: &v3.LoadBalancerServiceHealthcheck{
					Interval: int64(func() time.Duration {
						d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
						return d
					}().Seconds()),
					Mode:    defaultNLBServiceHealthcheckMode,
					Port:    int64(k8sServicePortNodePort),
					Retries: defaultNLBServiceHealthcheckRetries,
					TlsSNI:  "",
					Timeout: int64(func() time.Duration {
						d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
						return d
					}().Seconds()),
				},
				InstancePool: &v3.InstancePool{
					ID: testNLBServiceInstancePoolID,
				},
				Name:       nlbServicePortName,
				Port:       int64(k8sServicePortPort),
				Protocol:   testNLBServiceProtocol,
				Strategy:   testNLBServiceStrategy,
				TargetPort: int64(k8sServicePortNodePort),
			}},
		}

		expectedNLBService = &v3.LoadBalancerService{
			Healthcheck: &v3.LoadBalancerServiceHealthcheck{
				Interval: int64(func() time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
					return d
				}().Seconds()),
				Mode:    defaultNLBServiceHealthcheckMode,
				Port:    int64(k8sServicePortNodePort),
				Retries: defaultNLBServiceHealthcheckRetries,
				TlsSNI:  "",
				Timeout: int64(func() time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
					return d
				}().Seconds()),
			},
			InstancePool: &v3.InstancePool{
				ID: testNLBServiceInstancePoolID,
			},
			Name:       nlbServicePortName,
			Port:       int64(k8sServicePortPort),
			Protocol:   testNLBServiceProtocol,
			Strategy:   testNLBServiceStrategy,
			TargetPort: int64(k8sServicePortNodePort),
		}

		expectedStatus = &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}},
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(&v3.Instance{
			ID: testInstanceID,
			Manager: &v3.Manager{
				ID:   testNLBServiceInstancePoolID,
				Type: "instance-pool",
			},
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancer", ts.p.ctx, ts.p.zone, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbCreated = true
			ts.Require().Equal(args.Get(2), expectedNLB)
		}).
		Return(&v3.LoadBalancer{
			ID:   testNLBID,
			IP:   testNLBIPaddressP,
			Name: testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(&v3.LoadBalancer{
			Description: testNLBDescription,
			ID:          testNLBID,
			Name:        testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbServiceCreated = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(&v3.LoadBalancerService{ID: testNLBServiceID}, nil)

	ts.p.kclient = fake.NewSimpleClientset(service)

	status, err := ts.p.loadBalancer.EnsureLoadBalancer(
		ts.p.ctx,
		"",
		service,
		[]*v1.Node{{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID.String()}},
		}})
	ts.Require().NoError(err)
	ts.Require().Equal(expectedStatus, status)
	ts.Require().True(nlbCreated)
	ts.Require().True(nlbServiceCreated)

	// Testing creation error with an NLB annotated "external":

	delete(service.Annotations, annotationLoadBalancerID)
	service.Annotations[annotationLoadBalancerExternal] = "true"

	_, err = ts.p.loadBalancer.EnsureLoadBalancer(
		ts.p.ctx,
		"",
		service,
		[]*v1.Node{{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID.String()}},
		}})
	ts.Require().Error(err)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancer_reuse() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		nlbServiceCreated             = false

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: metav1.NamespaceDefault,
				UID:       types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerDescription: testNLBDescription,
					annotationLoadBalancerID:          string(testNLBID),
					annotationLoadBalancerName:        testNLBName,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol: v1.ProtocolTCP,
					Port:     int32(k8sServicePortPort),
					NodePort: int32(k8sServicePortNodePort),
				}},
			},
		}

		expectedNLBService = &v3.LoadBalancerService{
			Healthcheck: &v3.LoadBalancerServiceHealthcheck{
				Interval: int64(func() time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
					return d
				}().Seconds()),
				Mode:    defaultNLBServiceHealthcheckMode,
				Port:    int64(k8sServicePortNodePort),
				Retries: defaultNLBServiceHealthcheckRetries,
				TlsSNI:  "",
				Timeout: int64(func() time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
					return d
				}().Seconds()),
			},
			InstancePool: &v3.InstancePool{
				ID: testNLBServiceInstancePoolID,
			},
			Name:       nlbServicePortName,
			Port:       int64(k8sServicePortPort),
			Protocol:   testNLBServiceProtocol,
			Strategy:   testNLBServiceStrategy,
			TargetPort: int64(k8sServicePortNodePort),
		}

		expectedStatus = &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}},
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(&v3.Instance{
			ID: testInstanceID,
			Manager: &v3.Manager{
				ID:   testNLBServiceInstancePoolID,
				Type: "instance-pool",
			},
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(&v3.LoadBalancer{
			Description: testNLBDescription,
			ID:          testNLBID,
			IP:          testNLBIPaddressP,
			Name:        testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbServiceCreated = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(&v3.LoadBalancerService{ID: testNLBServiceID}, nil)

	ts.p.kclient = fake.NewSimpleClientset(service)

	status, err := ts.p.loadBalancer.EnsureLoadBalancer(
		ts.p.ctx,
		"",
		service,
		[]*v1.Node{{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{Name: testInstanceName},
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID.String()}},
		}})
	ts.Require().NoError(err)
	ts.Require().Equal(expectedStatus, status)
	ts.Require().True(nlbServiceCreated)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_EnsureLoadBalancerDeleted() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		nlbDeleted                    = false
		nlbServiceDeleted             = false

		expectedNLB = &v3.LoadBalancer{
			ID:   testNLBID,
			Name: testNLBName,
			Services: []v3.LoadBalancerService{{
				Name: nlbServicePortName,
				Port: int64(k8sServicePortPort),
			}},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: metav1.NamespaceDefault,
				UID:       types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:   string(testNLBID),
					annotationLoadBalancerName: testNLBName,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol: v1.ProtocolTCP,
					Port:     int32(k8sServicePortPort),
					NodePort: int32(k8sServicePortNodePort),
				}},
			},
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(expectedNLB, nil)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbServiceDeleted = true
			ts.Require().Equal(args.Get(3), expectedNLB.Services[0])
		}).
		Return(nil)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteNetworkLoadBalancer", ts.p.ctx, ts.p.zone, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbDeleted = true
			ts.Require().Equal(args.Get(2), expectedNLB)
		}).
		Return(nil)

	ts.p.kclient = fake.NewSimpleClientset(service)

	err := ts.p.loadBalancer.EnsureLoadBalancerDeleted(ts.p.ctx, "", service)
	ts.Require().NoError(err)
	ts.Require().True(nlbDeleted)
	ts.Require().True(nlbServiceDeleted)

	// Testing non-deletion with an NLB annotated "external":

	service.Annotations[annotationLoadBalancerExternal] = "true"
	nlbDeleted = false

	err = ts.p.loadBalancer.EnsureLoadBalancerDeleted(ts.p.ctx, "", service)
	ts.Require().NoError(err)
	ts.Require().False(nlbDeleted)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_GetLoadBalancer() {
	expectedStatus := &v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}},
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(&v3.LoadBalancer{
			ID:   testNLBID,
			IP:   testNLBIPaddressP,
			Name: testNLBName,
		}, nil)

	actualStatus, exists, err := ts.p.loadBalancer.GetLoadBalancer(ts.p.ctx, "", &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationLoadBalancerID: testNLBID.String(),
			},
		},
	})
	ts.Require().Equal(expectedStatus, actualStatus)
	ts.Require().True(exists)
	ts.Require().NoError(err)

	// Non-existent NLB

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, mock.Anything).
		Return(new(v3.LoadBalancer), errLoadBalancerNotFound)

	_, exists, err = ts.p.loadBalancer.GetLoadBalancer(ts.p.ctx, "", &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{},
		},
	})
	ts.Require().False(exists)
	ts.Require().NoError(err)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_GetLoadBalancerName() {
	testServiceUID := ts.randomID()

	type args struct {
		service *v1.Service
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "from annotations",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						UID: types.UID(testServiceUID),
						Annotations: map[string]string{
							annotationLoadBalancerName: testNLBName,
						},
					},
				},
			},
			want: testNLBName,
		},
		{
			name: "from service UID (default)",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						UID:         types.UID(testServiceUID),
						Annotations: map[string]string{},
					},
				},
			},
			want: "k8s-" + testServiceUID,
		},
	}

	for _, tt := range tests {
		ts.T().Run(tt.name, func(_ *testing.T) {
			actual := ts.p.loadBalancer.GetLoadBalancerName(ts.p.ctx, "", tt.args.service)
			if actual != tt.want {
				ts.T().Errorf("GetLoadBalancerName() = %v, want %v", actual, tt.want)
			}
		})
	}
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_UpdateLoadBalancer() {
	ts.T().Skip("wraps loadBalancer.updateLoadBalancer()")
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_fetchLoadBalancer() {
	expected := &v3.LoadBalancer{
		ID:   testNLBID,
		Name: testNLBName,
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(expected, nil)

	actual, err := ts.p.loadBalancer.(*loadBalancer).fetchLoadBalancer(ts.p.ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationLoadBalancerID: testNLBID.String(),
			},
		},
	})
	ts.Require().Equal(expected, actual)
	ts.Require().NoError(err)

	// Non-existent NLB

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, "lolnope").
		Return(new(v3.LoadBalancer), errLoadBalancerNotFound)

	_, err = ts.p.loadBalancer.(*loadBalancer).fetchLoadBalancer(ts.p.ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationLoadBalancerID: "lolnope",
			},
		},
	})
	ts.Require().ErrorIs(err, errLoadBalancerNotFound)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_patchAnnotation() {
	type args struct {
		service *v1.Service
		k       string
		v       string
	}

	tests := []struct {
		name    string
		args    args
		want    *v1.Service
		wantErr bool
	}{
		{
			name: "with nil annotations",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: metav1.NamespaceDefault,
					},
				},
				k: annotationLoadBalancerServiceInstancePoolID,
				v: testNLBServiceInstancePoolID.String(),
			},
			want: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
					},
				},
			},
		},
		{
			name: "with existing annotations",
			args: args{
				service: &v1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: metav1.NamespaceDefault,
						Annotations: map[string]string{
							annotationLoadBalancerName: testNLBName,
						},
					},
				},
				k: annotationLoadBalancerServiceInstancePoolID,
				v: testNLBServiceInstancePoolID.String(),
			},
			want: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						annotationLoadBalancerName:                  testNLBName,
						annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
					},
				},
			},
		},
	}

	ts.p.kclient = fake.NewSimpleClientset(&v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: metav1.NamespaceDefault,
			Annotations: map[string]string{
				annotationLoadBalancerName:                  testNLBName,
				annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
			},
		},
	})

	for _, tt := range tests {
		ts.T().Run(tt.name, func(t *testing.T) {
			err := ts.p.loadBalancer.(*loadBalancer).patchAnnotation(
				ts.p.ctx,
				tt.args.service,
				tt.args.k,
				tt.args.v,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("patchAnnotation() error = %v, wantErr %v", err, tt.wantErr)
			}
			ts.Require().Equal(tt.want, tt.args.service)
		})
	}
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_updateLoadBalancer_create() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		created                       = false

		currentNLB = &v3.LoadBalancer{
			CreatedAT: testNLBCreatedAt,
			ID:        testNLBID,
			IP:        testNLBIPaddressP,
			Name:      testNLBName,
			Services:  []v3.LoadBalancerService{},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:                    currentNLB.ID.String(),
					annotationLoadBalancerName:                  currentNLB.Name,
					annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol: v1.ProtocolTCP,
					Port:     int32(k8sServicePortPort),
					NodePort: int32(k8sServicePortNodePort),
				}},
			},
		}
	)

	expectedNLBService := &v3.LoadBalancerService{
		Healthcheck: &v3.LoadBalancerServiceHealthcheck{
			Interval: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return d
			}().Seconds()),
			Mode:    defaultNLBServiceHealthcheckMode,
			Port:    int64(k8sServicePortNodePort),
			Retries: defaultNLBServiceHealthcheckRetries,
			TlsSNI:  "",
			Timeout: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return d
			}().Seconds()),
		},
		InstancePool: &v3.InstancePool{
			ID: testNLBServiceInstancePoolID,
		},
		Name:       nlbServicePortName,
		Port:       int64(k8sServicePortPort),
		Protocol:   testNLBServiceProtocol,
		Strategy:   testNLBServiceStrategy,
		TargetPort: int64(k8sServicePortNodePort),
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(currentNLB, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			created = true
			ts.Require().Equal(args.Get(2), currentNLB)
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(&v3.LoadBalancerService{ID: testNLBServiceID}, nil)

	ts.Require().NoError(ts.p.loadBalancer.(*loadBalancer).updateLoadBalancer(ts.p.ctx, service))
	ts.Require().True(created)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_updateLoadBalancer_update() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		updated                       = false

		currentNLB = &v3.LoadBalancer{
			CreatedAT: testNLBCreatedAt,
			ID:        testNLBID,
			IP:        testNLBIPaddressP,
			Name:      testNLBName,
			Services: []v3.LoadBalancerService{
				{
					Healthcheck: &v3.LoadBalancerServiceHealthcheck{
						Interval: int64(func() time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
							return d
						}().Seconds()),
						Mode:    defaultNLBServiceHealthcheckMode,
						Port:    int64(k8sServicePortNodePort),
						Retries: defaultNLBServiceHealthcheckRetries,
						TlsSNI:  "",
						Timeout: int64(func() time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
							return d
						}().Seconds()),
					},
					InstancePool: &v3.InstancePool{
						ID: testNLBServiceInstancePoolID,
					},
					Name:       nlbServicePortName,
					Port:       int64(k8sServicePortPort),
					Protocol:   testNLBServiceProtocol,
					Strategy:   testNLBServiceStrategy,
					TargetPort: int64(k8sServicePortNodePort),
				},
			},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:                    currentNLB.ID.String(),
					annotationLoadBalancerName:                  currentNLB.Name,
					annotationLoadBalancerServiceDescription:    testNLBServiceDescription,
					annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID.String(),
					annotationLoadBalancerServiceName:           testNLBServiceName,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{{
					Protocol: v1.ProtocolTCP,
					Port:     int32(k8sServicePortPort),
					NodePort: int32(k8sServicePortNodePort),
				}},
			},
		}
	)

	expectedNLBService := &v3.LoadBalancerService{
		Description: testNLBServiceDescription,
		Healthcheck: &v3.LoadBalancerServiceHealthcheck{
			Interval: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return d
			}().Seconds()),
			Mode:    defaultNLBServiceHealthcheckMode,
			Port:    int64(k8sServicePortNodePort),
			Retries: defaultNLBServiceHealthcheckRetries,
			TlsSNI:  "",
			Timeout: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return d
			}().Seconds()),
		},
		InstancePool: &v3.InstancePool{
			ID: testNLBServiceInstancePoolID,
		},
		Name:       nlbServicePortName,
		Port:       int64(k8sServicePortPort),
		Protocol:   testNLBServiceProtocol,
		Strategy:   testNLBServiceStrategy,
		TargetPort: int64(k8sServicePortNodePort),
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(currentNLB, nil)

	ts.p.client.(*exoscaleClientMock).
		On("UpdateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			updated = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(nil)

	ts.Require().NoError(ts.p.loadBalancer.(*loadBalancer).updateLoadBalancer(ts.p.ctx, service))
	ts.Require().True(updated)
}

func (ts *exoscaleCCMTestSuite) Test_loadBalancer_updateLoadBalancer_delete() {
	var (
		k8sServiceUID                 = ts.randomID()
		k8sServicePortPort     uint16 = 80
		k8sServicePortNodePort uint16 = 32672
		nlbServicePortName            = fmt.Sprintf("%s-%d", k8sServiceUID, k8sServicePortPort)
		deleted                       = false

		currentNLB = &v3.LoadBalancer{
			CreatedAT: testNLBCreatedAt,
			ID:        testNLBID,
			IP:        testNLBIPaddressP,
			Name:      testNLBName,
			Services: []v3.LoadBalancerService{
				{
					Healthcheck: &v3.LoadBalancerServiceHealthcheck{
						Interval: int64(func() time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
							return d
						}().Seconds()),
						Mode:    defaultNLBServiceHealthcheckMode,
						Port:    int64(k8sServicePortNodePort),
						Retries: defaultNLBServiceHealthcheckRetries,
						TlsSNI:  "",
						Timeout: int64(func() time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
							return d
						}().Seconds()),
					},
					InstancePool: &v3.InstancePool{
						ID: testNLBServiceInstancePoolID,
					},
					Name:       nlbServicePortName,
					Port:       int64(k8sServicePortPort),
					Protocol:   testNLBServiceProtocol,
					Strategy:   testNLBServiceStrategy,
					TargetPort: int64(k8sServicePortNodePort),
				},
			},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:   currentNLB.ID.String(),
					annotationLoadBalancerName: currentNLB.Name,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{},
			},
		}
	)

	expectedNLBService := &v3.LoadBalancerService{
		Healthcheck: &v3.LoadBalancerServiceHealthcheck{
			Interval: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return d
			}().Seconds()),
			Mode:    defaultNLBServiceHealthcheckMode,
			Port:    int64(k8sServicePortNodePort),
			Retries: defaultNLBServiceHealthcheckRetries,
			TlsSNI:  "",
			Timeout: int64(func() time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return d
			}().Seconds()),
		},
		InstancePool: &v3.InstancePool{
			ID: testNLBServiceInstancePoolID,
		},
		Name:       nlbServicePortName,
		Port:       int64(k8sServicePortPort),
		Protocol:   testNLBServiceProtocol,
		Strategy:   testNLBServiceStrategy,
		TargetPort: int64(k8sServicePortNodePort),
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(currentNLB, nil)

	ts.p.client.(*exoscaleClientMock).
		On("DeleteNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			deleted = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(nil)

	ts.Require().NoError(ts.p.loadBalancer.(*loadBalancer).updateLoadBalancer(ts.p.ctx, service))
	ts.Require().True(deleted)
}

func Test_buildLoadBalancerFromAnnotations(t *testing.T) {
	var (
		serviceUID                      = "901a4773-b836-409d-9364-b855b7b38c22"
		servicePortProtocol             = v1.ProtocolTCP
		servicePortHTTPName             = "http"
		servicePortHTTPPort      uint16 = 80
		servicePortHTTPNodePort  uint16 = 32058
		servicePortHTTPSName            = "https"
		servicePortHTTPSPort     uint16 = 443
		servicePortHTTPSNodePort uint16 = 32059
		serviceHTTPDefaultName          = fmt.Sprintf("%s-%d", serviceUID, servicePortHTTPPort)
		serviceHTTPSDefaultName         = fmt.Sprintf("%s-%d", serviceUID, servicePortHTTPSPort)

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(serviceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:                         testNLBID.String(),
					annotationLoadBalancerName:                       testNLBName,
					annotationLoadBalancerDescription:                testNLBDescription,
					annotationLoadBalancerServiceName:                testNLBServiceName,
					annotationLoadBalancerServiceDescription:         testNLBServiceDescription,
					annotationLoadBalancerServiceStrategy:            string(testNLBServiceStrategy),
					annotationLoadBalancerServiceInstancePoolID:      string(testNLBServiceInstancePoolID),
					annotationLoadBalancerServiceHealthCheckMode:     string(testNLBServiceHealthcheckMode),
					annotationLoadBalancerServiceHealthCheckURI:      testNLBServiceHealthcheckURI,
					annotationLoadBalancerServiceHealthCheckInterval: fmt.Sprint(testNLBServiceHealthcheckInterval),
					annotationLoadBalancerServiceHealthCheckTimeout:  fmt.Sprint(testNLBServiceHealthcheckTimeout),
					annotationLoadBalancerServiceHealthCheckRetries:  fmt.Sprint(testNLBServiceHealthcheckRetries),
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:     servicePortHTTPName,
						Protocol: servicePortProtocol,
						Port:     int32(servicePortHTTPPort),
						NodePort: int32(servicePortHTTPNodePort),
					},
					{
						Name:     servicePortHTTPSName,
						Protocol: servicePortProtocol,
						Port:     int32(servicePortHTTPSPort),
						NodePort: int32(servicePortHTTPSNodePort),
					},
				},
			},
		}
	)

	expected := &v3.LoadBalancer{
		ID:          testNLBID,
		Name:        testNLBName,
		Description: testNLBDescription,
		Services: []v3.LoadBalancerService{
			{
				Name: serviceHTTPDefaultName,
				InstancePool: &v3.InstancePool{
					ID: testNLBServiceInstancePoolID,
				},
				Protocol:   testNLBServiceProtocol,
				Port:       int64(servicePortHTTPPort),
				TargetPort: int64(servicePortHTTPNodePort),
				Strategy:   testNLBServiceStrategy,
				Healthcheck: &v3.LoadBalancerServiceHealthcheck{
					Mode:     testNLBServiceHealthcheckMode,
					Port:     int64(servicePortHTTPNodePort),
					URI:      testNLBServiceHealthcheckURI,
					Interval: int64(testNLBServiceHealthcheckInterval.Seconds()),
					Timeout:  int64(testNLBServiceHealthcheckTimeout.Seconds()),
					Retries:  testNLBServiceHealthcheckRetries,
				},
			},
			{
				Name: serviceHTTPSDefaultName,
				InstancePool: &v3.InstancePool{
					ID: testNLBServiceInstancePoolID,
				},
				Protocol:   testNLBServiceProtocol,
				Port:       int64(servicePortHTTPSPort),
				TargetPort: int64(servicePortHTTPSNodePort),
				Strategy:   testNLBServiceStrategy,
				Healthcheck: &v3.LoadBalancerServiceHealthcheck{
					Mode:     testNLBServiceHealthcheckMode,
					Port:     int64(servicePortHTTPSNodePort),
					URI:      testNLBServiceHealthcheckURI,
					Interval: int64(testNLBServiceHealthcheckInterval.Seconds()),
					Timeout:  int64(testNLBServiceHealthcheckTimeout.Seconds()),
					Retries:  testNLBServiceHealthcheckRetries,
				},
			},
		},
	}

	actual, err := buildLoadBalancerFromAnnotations(service)
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	// Variant: with a single service, NLB service name/description can be overridden via annotation.
	service.Spec.Ports = service.Spec.Ports[:1]
	expected.Services = expected.Services[:1]
	expected.Services[0].Name = testNLBServiceName
	expected.Services[0].Description = testNLBServiceDescription
	actual, err = buildLoadBalancerFromAnnotations(service)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func Test_isLoadBalancerUpdated(t *testing.T) {
	tests := []struct {
		name      string
		lbA       *v3.LoadBalancer
		lbB       *v3.LoadBalancer
		assertion require.BoolAssertionFunc
	}{
		{
			"no change",
			&v3.LoadBalancer{Name: testNLBName, Description: testNLBDescription},
			&v3.LoadBalancer{Name: testNLBName, Description: testNLBDescription},
			require.False,
		},
		{
			"description updated",
			&v3.LoadBalancer{Name: testNLBName},
			&v3.LoadBalancer{Name: testNLBName, Description: testNLBDescription},
			require.True,
		},
		{
			"name updated",
			&v3.LoadBalancer{Description: testNLBDescription},
			&v3.LoadBalancer{Name: testNLBName, Description: testNLBDescription},
			require.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertion(t, isLoadBalancerUpdated(tt.lbA, tt.lbB))
		})
	}
}

func Test_isLoadBalancerServiceUpdated(t *testing.T) {
	tests := []struct {
		name      string
		svcA      *v3.LoadBalancerService
		svcB      *v3.LoadBalancerService
		assertion require.BoolAssertionFunc
	}{
		{
			"no change",
			&v3.LoadBalancerService{Name: testNLBServiceName, Description: testNLBServiceDescription},
			&v3.LoadBalancerService{Name: testNLBServiceName, Description: testNLBServiceDescription},
			require.False,
		},
		{
			"description updated",
			&v3.LoadBalancerService{Name: testNLBServiceName},
			&v3.LoadBalancerService{Name: testNLBServiceName, Description: testNLBServiceDescription},
			require.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertion(t, isLoadBalancerServiceUpdated(tt.svcA, tt.svcB))
		})
	}
}

func Test_getAnnotation(t *testing.T) {
	type args struct {
		service      *v1.Service
		annotation   string
		defaultValue string
	}

	var (
		testDefaultValue = "fallback"
		tests            = []struct {
			name string
			args args
			want *string
		}{
			{
				name: "fallback to default value",
				args: args{
					service: &v1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								annotationLoadBalancerID: testNLBID.String(),
							},
						},
					},
					annotation:   "lolnope",
					defaultValue: testDefaultValue,
				},
				want: &testDefaultValue,
			},
			{
				name: "ok",
				args: args{
					service: &v1.Service{
						ObjectMeta: metav1.ObjectMeta{
							Annotations: map[string]string{
								annotationLoadBalancerID: testNLBID.String(),
							},
						},
					},
					annotation: annotationLoadBalancerID,
				},
				want: v3.Ptr(testNLBID.String()),
			},
		}
	)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := getAnnotation(tt.args.service, tt.args.annotation, tt.args.defaultValue)
			if !reflect.DeepEqual(actual, tt.want) {
				t.Errorf("getAnnotation() = %v, want %v", actual, tt.want)
			}
		})
	}
}
