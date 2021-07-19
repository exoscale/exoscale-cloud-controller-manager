package exoscale

import (
	"fmt"
	"strings"
	"testing"
	"time"

	egoscale "github.com/exoscale/egoscale/v2"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	testNLBDescription                      = "nlb-description"
	testNLBID                               = "61a38660-ce67-4649-abef-284aa735d49d"
	testNLBName                             = "nlb-name"
	testNLBServiceDescription               = "nlb-service-description"
	testNLBServiceHealthcheckInterval       = 10 * time.Second
	testNLBServiceHealthcheckMode           = "http"
	testNLBServiceHealthcheckRetries  int64 = 2
	testNLBServiceHealthcheckTimeout        = 5 * time.Second
	testNLBServiceHealthcheckURI            = "/health"
	testNLBServiceInstancePoolID            = "1ca4a029-3df9-4a20-8a68-437d5de2f5fb"
	testNLBServiceName                      = "nlb-service-name"
	testNLBServiceProtocol                  = strings.ToLower(string(v1.ProtocolTCP))
	testNLBServiceStrategy                  = "round-robin"
)

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
