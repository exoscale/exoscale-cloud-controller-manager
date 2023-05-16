package exoscale

import (
	"fmt"
	"net"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	egoscale "github.com/exoscale/egoscale/v2"
)

var (
	testNLBCreatedAt                        = time.Now().UTC()
	testNLBDescription                      = new(exoscaleCCMTestSuite).randomString(10)
	testNLBID                               = new(exoscaleCCMTestSuite).randomID()
	testNLBIPaddress                        = "1.2.3.4"
	testNLBIPaddressP                       = net.ParseIP(testNLBIPaddress)
	testNLBName                             = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceDescription               = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceHealthcheckInterval       = 10 * time.Second
	testNLBServiceHealthcheckMode           = "http"
	testNLBServiceHealthcheckRetries  int64 = 2
	testNLBServiceHealthcheckTimeout        = 5 * time.Second
	testNLBServiceHealthcheckURI            = "/health"
	testNLBServiceID                        = new(exoscaleCCMTestSuite).randomID()
	testNLBServiceInstancePoolID            = new(exoscaleCCMTestSuite).randomID()
	testNLBServiceName                      = new(exoscaleCCMTestSuite).randomString(10)
	testNLBServiceProtocol                  = strings.ToLower(string(v1.ProtocolTCP))
	testNLBServiceStrategy                  = "round-robin"
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

		expectedNLB = &egoscale.NetworkLoadBalancer{
			Description: &testNLBDescription,
			Name:        &testNLBName,
			Services: []*egoscale.NetworkLoadBalancerService{{
				Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
					Interval: func() *time.Duration {
						d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
						return &d
					}(),
					Mode:    &defaultNLBServiceHealthcheckMode,
					Port:    &k8sServicePortNodePort,
					Retries: &defaultNLBServiceHealthcheckRetries,
					TLSSNI:  nil,
					Timeout: func() *time.Duration {
						d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
						return &d
					}(),
				},
				InstancePoolID: &testNLBServiceInstancePoolID,
				Name:           &nlbServicePortName,
				Port:           &k8sServicePortPort,
				Protocol:       &testNLBServiceProtocol,
				Strategy:       &testNLBServiceStrategy,
				TargetPort:     &k8sServicePortNodePort,
			}},
		}

		expectedNLBService = &egoscale.NetworkLoadBalancerService{
			Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
				Interval: func() *time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
					return &d
				}(),
				Mode:    &defaultNLBServiceHealthcheckMode,
				Port:    &k8sServicePortNodePort,
				Retries: &defaultNLBServiceHealthcheckRetries,
				TLSSNI:  nil,
				Timeout: func() *time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
					return &d
				}(),
			},
			InstancePoolID: &testNLBServiceInstancePoolID,
			Name:           &nlbServicePortName,
			Port:           &k8sServicePortPort,
			Protocol:       &testNLBServiceProtocol,
			Strategy:       &testNLBServiceStrategy,
			TargetPort:     &k8sServicePortNodePort,
		}

		expectedStatus = &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}},
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(&egoscale.Instance{
			ID: &testInstanceID,
			Manager: &egoscale.InstanceManager{
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
		Return(&egoscale.NetworkLoadBalancer{
			ID:        &testNLBID,
			IPAddress: &testNLBIPaddressP,
			Name:      &testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(&egoscale.NetworkLoadBalancer{
			Description: &testNLBDescription,
			ID:          &testNLBID,
			Name:        &testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbServiceCreated = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(&egoscale.NetworkLoadBalancerService{ID: &testNLBServiceID}, nil)

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
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID}},
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
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID}},
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
					annotationLoadBalancerID:          testNLBID,
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

		expectedNLBService = &egoscale.NetworkLoadBalancerService{
			Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
				Interval: func() *time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
					return &d
				}(),
				Mode:    &defaultNLBServiceHealthcheckMode,
				Port:    &k8sServicePortNodePort,
				Retries: &defaultNLBServiceHealthcheckRetries,
				TLSSNI:  nil,
				Timeout: func() *time.Duration {
					d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
					return &d
				}(),
			},
			InstancePoolID: &testNLBServiceInstancePoolID,
			Name:           &nlbServicePortName,
			Port:           &k8sServicePortPort,
			Protocol:       &testNLBServiceProtocol,
			Strategy:       &testNLBServiceStrategy,
			TargetPort:     &k8sServicePortNodePort,
		}

		expectedStatus = &v1.LoadBalancerStatus{
			Ingress: []v1.LoadBalancerIngress{{IP: testNLBIPaddress}},
		}
	)

	ts.p.client.(*exoscaleClientMock).
		On("GetInstance", ts.p.ctx, ts.p.zone, testInstanceID).
		Return(&egoscale.Instance{
			ID: &testInstanceID,
			Manager: &egoscale.InstanceManager{
				ID:   testNLBServiceInstancePoolID,
				Type: "instance-pool",
			},
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(&egoscale.NetworkLoadBalancer{
			Description: &testNLBDescription,
			ID:          &testNLBID,
			IPAddress:   &testNLBIPaddressP,
			Name:        &testNLBName,
		}, nil)

	ts.p.client.(*exoscaleClientMock).
		On("CreateNetworkLoadBalancerService", ts.p.ctx, ts.p.zone, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			nlbServiceCreated = true
			ts.Require().Equal(args.Get(3), expectedNLBService)
		}).
		Return(&egoscale.NetworkLoadBalancerService{ID: &testNLBServiceID}, nil)

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
			Status:     v1.NodeStatus{NodeInfo: v1.NodeSystemInfo{SystemUUID: testInstanceID}},
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

		expectedNLB = &egoscale.NetworkLoadBalancer{
			ID:   &testNLBID,
			Name: &testNLBName,
			Services: []*egoscale.NetworkLoadBalancerService{{
				Name: &nlbServicePortName,
				Port: &k8sServicePortPort,
			}},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: metav1.NamespaceDefault,
				UID:       types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:   testNLBID,
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
		Return(&egoscale.NetworkLoadBalancer{
			ID:        &testNLBID,
			IPAddress: &testNLBIPaddressP,
			Name:      &testNLBName,
		}, nil)

	actualStatus, exists, err := ts.p.loadBalancer.GetLoadBalancer(ts.p.ctx, "", &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationLoadBalancerID: testNLBID,
			},
		},
	})
	ts.Require().Equal(expectedStatus, actualStatus)
	ts.Require().True(exists)
	ts.Require().NoError(err)

	// Non-existent NLB

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, mock.Anything).
		Return(new(egoscale.NetworkLoadBalancer), errLoadBalancerNotFound)

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
	expected := &egoscale.NetworkLoadBalancer{
		ID:   &testNLBID,
		Name: &testNLBName,
	}

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, testNLBID).
		Return(expected, nil)

	actual, err := ts.p.loadBalancer.(*loadBalancer).fetchLoadBalancer(ts.p.ctx, &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				annotationLoadBalancerID: testNLBID,
			},
		},
	})
	ts.Require().Equal(expected, actual)
	ts.Require().NoError(err)

	// Non-existent NLB

	ts.p.client.(*exoscaleClientMock).
		On("GetNetworkLoadBalancer", ts.p.ctx, ts.p.zone, "lolnope").
		Return(new(egoscale.NetworkLoadBalancer), errLoadBalancerNotFound)

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
				v: testNLBServiceInstancePoolID,
			},
			want: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID,
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
				v: testNLBServiceInstancePoolID,
			},
			want: &v1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: metav1.NamespaceDefault,
					Annotations: map[string]string{
						annotationLoadBalancerName:                  testNLBName,
						annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID,
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
				annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID,
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

		currentNLB = &egoscale.NetworkLoadBalancer{
			CreatedAt: &testNLBCreatedAt,
			ID:        &testNLBID,
			IPAddress: &testNLBIPaddressP,
			Name:      &testNLBName,
			Services:  []*egoscale.NetworkLoadBalancerService{},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:                    *currentNLB.ID,
					annotationLoadBalancerName:                  *currentNLB.Name,
					annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID,
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

	expectedNLBService := &egoscale.NetworkLoadBalancerService{
		Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
			Interval: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return &d
			}(),
			Mode:    &defaultNLBServiceHealthcheckMode,
			Port:    &k8sServicePortNodePort,
			Retries: &defaultNLBServiceHealthcheckRetries,
			TLSSNI:  nil,
			Timeout: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return &d
			}(),
		},
		InstancePoolID: &testNLBServiceInstancePoolID,
		Name:           &nlbServicePortName,
		Port:           &k8sServicePortPort,
		Protocol:       &testNLBServiceProtocol,
		Strategy:       &testNLBServiceStrategy,
		TargetPort:     &k8sServicePortNodePort,
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
		Return(&egoscale.NetworkLoadBalancerService{ID: &testNLBServiceID}, nil)

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

		currentNLB = &egoscale.NetworkLoadBalancer{
			CreatedAt: &testNLBCreatedAt,
			ID:        &testNLBID,
			IPAddress: &testNLBIPaddressP,
			Name:      &testNLBName,
			Services: []*egoscale.NetworkLoadBalancerService{
				{
					Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
						Interval: func() *time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
							return &d
						}(),
						Mode:    &defaultNLBServiceHealthcheckMode,
						Port:    &k8sServicePortNodePort,
						Retries: &defaultNLBServiceHealthcheckRetries,
						TLSSNI:  nil,
						Timeout: func() *time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
							return &d
						}(),
					},
					ID:             &testNLBServiceID,
					InstancePoolID: &testNLBServiceInstancePoolID,
					Name:           &nlbServicePortName,
					Port:           &k8sServicePortPort,
					Protocol:       &testNLBServiceProtocol,
					Strategy:       &testNLBServiceStrategy,
					TargetPort:     &k8sServicePortNodePort,
				},
			},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:                    *currentNLB.ID,
					annotationLoadBalancerName:                  *currentNLB.Name,
					annotationLoadBalancerServiceDescription:    testNLBServiceDescription,
					annotationLoadBalancerServiceInstancePoolID: testNLBServiceInstancePoolID,
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

	expectedNLBService := &egoscale.NetworkLoadBalancerService{
		Description: &testNLBServiceDescription,
		Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
			Interval: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return &d
			}(),
			Mode:    &defaultNLBServiceHealthcheckMode,
			Port:    &k8sServicePortNodePort,
			Retries: &defaultNLBServiceHealthcheckRetries,
			TLSSNI:  nil,
			Timeout: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return &d
			}(),
		},
		ID:             &testNLBServiceID,
		InstancePoolID: &testNLBServiceInstancePoolID,
		Name:           &testNLBServiceName,
		Port:           &k8sServicePortPort,
		Protocol:       &testNLBServiceProtocol,
		Strategy:       &testNLBServiceStrategy,
		TargetPort:     &k8sServicePortNodePort,
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

		currentNLB = &egoscale.NetworkLoadBalancer{
			CreatedAt: &testNLBCreatedAt,
			ID:        &testNLBID,
			IPAddress: &testNLBIPaddressP,
			Name:      &testNLBName,
			Services: []*egoscale.NetworkLoadBalancerService{
				{
					Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
						Interval: func() *time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
							return &d
						}(),
						Mode:    &defaultNLBServiceHealthcheckMode,
						Port:    &k8sServicePortNodePort,
						Retries: &defaultNLBServiceHealthcheckRetries,
						TLSSNI:  nil,
						Timeout: func() *time.Duration {
							d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
							return &d
						}(),
					},
					ID:             &testNLBServiceID,
					InstancePoolID: &testNLBServiceInstancePoolID,
					Name:           &nlbServicePortName,
					Port:           &k8sServicePortPort,
					Protocol:       &testNLBServiceProtocol,
					Strategy:       &testNLBServiceStrategy,
					TargetPort:     &k8sServicePortNodePort,
				},
			},
		}

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: types.UID(k8sServiceUID),
				Annotations: map[string]string{
					annotationLoadBalancerID:   *currentNLB.ID,
					annotationLoadBalancerName: *currentNLB.Name,
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{},
			},
		}
	)

	expectedNLBService := &egoscale.NetworkLoadBalancerService{
		Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
			Interval: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthcheckInterval)
				return &d
			}(),
			Mode:    &defaultNLBServiceHealthcheckMode,
			Port:    &k8sServicePortNodePort,
			Retries: &defaultNLBServiceHealthcheckRetries,
			TLSSNI:  nil,
			Timeout: func() *time.Duration {
				d, _ := time.ParseDuration(defaultNLBServiceHealthCheckTimeout)
				return &d
			}(),
		},
		ID:             &testNLBServiceID,
		InstancePoolID: &testNLBServiceInstancePoolID,
		Name:           &nlbServicePortName,
		Port:           &k8sServicePortPort,
		Protocol:       &testNLBServiceProtocol,
		Strategy:       &testNLBServiceStrategy,
		TargetPort:     &k8sServicePortNodePort,
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
					annotationLoadBalancerID:                         testNLBID,
					annotationLoadBalancerName:                       testNLBName,
					annotationLoadBalancerDescription:                testNLBDescription,
					annotationLoadBalancerServiceName:                testNLBServiceName,
					annotationLoadBalancerServiceDescription:         testNLBServiceDescription,
					annotationLoadBalancerServiceStrategy:            testNLBServiceStrategy,
					annotationLoadBalancerServiceInstancePoolID:      testNLBServiceInstancePoolID,
					annotationLoadBalancerServiceHealthCheckMode:     testNLBServiceHealthcheckMode,
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

	expected := &egoscale.NetworkLoadBalancer{
		ID:          &testNLBID,
		Name:        &testNLBName,
		Description: &testNLBDescription,
		Services: []*egoscale.NetworkLoadBalancerService{
			{
				Name:           &serviceHTTPDefaultName,
				InstancePoolID: &testNLBServiceInstancePoolID,
				Protocol:       &testNLBServiceProtocol,
				Port:           &servicePortHTTPPort,
				TargetPort:     &servicePortHTTPNodePort,
				Strategy:       &testNLBServiceStrategy,
				Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
					Mode:     &testNLBServiceHealthcheckMode,
					Port:     &servicePortHTTPNodePort,
					URI:      &testNLBServiceHealthcheckURI,
					Interval: &testNLBServiceHealthcheckInterval,
					Timeout:  &testNLBServiceHealthcheckTimeout,
					Retries:  &testNLBServiceHealthcheckRetries,
				},
			},
			{
				Name:           &serviceHTTPSDefaultName,
				InstancePoolID: &testNLBServiceInstancePoolID,
				Protocol:       &testNLBServiceProtocol,
				Port:           &servicePortHTTPSPort,
				TargetPort:     &servicePortHTTPSNodePort,
				Strategy:       &testNLBServiceStrategy,
				Healthcheck: &egoscale.NetworkLoadBalancerServiceHealthcheck{
					Mode:     &testNLBServiceHealthcheckMode,
					Port:     &servicePortHTTPSNodePort,
					URI:      &testNLBServiceHealthcheckURI,
					Interval: &testNLBServiceHealthcheckInterval,
					Timeout:  &testNLBServiceHealthcheckTimeout,
					Retries:  &testNLBServiceHealthcheckRetries,
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
	expected.Services[0].Name = &testNLBServiceName
	expected.Services[0].Description = &testNLBServiceDescription
	actual, err = buildLoadBalancerFromAnnotations(service)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func Test_isLoadBalancerUpdated(t *testing.T) {
	tests := []struct {
		name      string
		lbA       *egoscale.NetworkLoadBalancer
		lbB       *egoscale.NetworkLoadBalancer
		assertion require.BoolAssertionFunc
	}{
		{
			"no change",
			&egoscale.NetworkLoadBalancer{Name: &testNLBName, Description: &testNLBDescription},
			&egoscale.NetworkLoadBalancer{Name: &testNLBName, Description: &testNLBDescription},
			require.False,
		},
		{
			"description updated",
			&egoscale.NetworkLoadBalancer{Name: &testNLBName},
			&egoscale.NetworkLoadBalancer{Name: &testNLBName, Description: &testNLBDescription},
			require.True,
		},
		{
			"name updated",
			&egoscale.NetworkLoadBalancer{Description: &testNLBDescription},
			&egoscale.NetworkLoadBalancer{Name: &testNLBName, Description: &testNLBDescription},
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
		svcA      *egoscale.NetworkLoadBalancerService
		svcB      *egoscale.NetworkLoadBalancerService
		assertion require.BoolAssertionFunc
	}{
		{
			"no change",
			&egoscale.NetworkLoadBalancerService{Name: &testNLBServiceName, Description: &testNLBServiceDescription},
			&egoscale.NetworkLoadBalancerService{Name: &testNLBServiceName, Description: &testNLBServiceDescription},
			require.False,
		},
		{
			"description updated",
			&egoscale.NetworkLoadBalancerService{Name: &testNLBServiceName},
			&egoscale.NetworkLoadBalancerService{Name: &testNLBServiceName, Description: &testNLBServiceDescription},
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
								annotationLoadBalancerID: testNLBID,
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
								annotationLoadBalancerID: testNLBID,
							},
						},
					},
					annotation: annotationLoadBalancerID,
				},
				want: &testNLBID,
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
