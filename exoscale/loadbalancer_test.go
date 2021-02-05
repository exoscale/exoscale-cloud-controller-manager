package exoscale

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/exoscale/egoscale"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_buildLoadBalancerFromAnnotations(t *testing.T) {
	var (
		serviceHealthcheckInterval       = 10 * time.Second
		serviceHealthcheckTimeout        = 5 * time.Second
		serviceHealthcheckRetries  int64 = 2

		servicePortHTTPName            = "http"
		servicePortHTTPPort      int32 = 80
		servicePortHTTPNodePort  int32 = 32058
		servicePortHTTPProtocol        = v1.ProtocolTCP
		servicePortHTTPSName           = "https"
		servicePortHTTPSPort     int32 = 443
		servicePortHTTPSNodePort int32 = 32059
		servicePortHTTPSProtocol       = v1.ProtocolTCP

		service = &v1.Service{
			ObjectMeta: metav1.ObjectMeta{
				UID: "901a4773-b836-409d-9364-b855b7b38c22",
				Annotations: map[string]string{
					annotationLoadBalancerID:                         "61a38660-ce67-4649-abef-284aa735d49d",
					annotationLoadBalancerName:                       "nlb-name",
					annotationLoadBalancerDescription:                "nlb-description",
					annotationLoadBalancerServiceName:                "service-name",
					annotationLoadBalancerServiceDescription:         "service-description",
					annotationLoadBalancerServiceStrategy:            "round-robin",
					annotationLoadBalancerServiceInstancePoolID:      "1ca4a029-3df9-4a20-8a68-437d5de2f5fb",
					annotationLoadBalancerServiceHealthCheckMode:     "http",
					annotationLoadBalancerServiceHealthCheckURI:      "/health",
					annotationLoadBalancerServiceHealthCheckInterval: fmt.Sprint(serviceHealthcheckInterval),
					annotationLoadBalancerServiceHealthCheckTimeout:  fmt.Sprint(serviceHealthcheckTimeout),
					annotationLoadBalancerServiceHealthCheckRetries:  fmt.Sprint(serviceHealthcheckRetries),
				},
			},
			Spec: v1.ServiceSpec{
				Ports: []v1.ServicePort{
					{
						Name:     servicePortHTTPName,
						Protocol: servicePortHTTPProtocol,
						Port:     servicePortHTTPPort,
						NodePort: servicePortHTTPNodePort,
					},
					{
						Name:     servicePortHTTPSName,
						Protocol: servicePortHTTPSProtocol,
						Port:     servicePortHTTPSPort,
						NodePort: servicePortHTTPSNodePort,
					},
				},
			},
		}
	)

	expected := &egoscale.NetworkLoadBalancer{
		ID:          service.Annotations[annotationLoadBalancerID],
		Name:        service.Annotations[annotationLoadBalancerName],
		Description: service.Annotations[annotationLoadBalancerDescription],
		Services: []*egoscale.NetworkLoadBalancerService{
			{
				Name:           fmt.Sprintf("%s-%d", service.UID, servicePortHTTPPort),
				InstancePoolID: service.Annotations[annotationLoadBalancerServiceInstancePoolID],
				Protocol:       strings.ToLower(string(servicePortHTTPProtocol)),
				Port:           uint16(servicePortHTTPPort),
				TargetPort:     uint16(servicePortHTTPNodePort),
				Strategy:       service.Annotations[annotationLoadBalancerServiceStrategy],
				Healthcheck: egoscale.NetworkLoadBalancerServiceHealthcheck{
					Mode:     service.Annotations[annotationLoadBalancerServiceHealthCheckMode],
					Port:     uint16(servicePortHTTPNodePort),
					URI:      service.Annotations[annotationLoadBalancerServiceHealthCheckURI],
					Interval: serviceHealthcheckInterval,
					Timeout:  serviceHealthcheckTimeout,
					Retries:  serviceHealthcheckRetries,
				},
			},
			{
				Name:           fmt.Sprintf("%s-%d", service.UID, servicePortHTTPSPort),
				InstancePoolID: service.Annotations[annotationLoadBalancerServiceInstancePoolID],
				Protocol:       strings.ToLower(string(servicePortHTTPSProtocol)),
				Port:           uint16(servicePortHTTPSPort),
				TargetPort:     uint16(servicePortHTTPSNodePort),
				Strategy:       service.Annotations[annotationLoadBalancerServiceStrategy],
				Healthcheck: egoscale.NetworkLoadBalancerServiceHealthcheck{
					Mode:     service.Annotations[annotationLoadBalancerServiceHealthCheckMode],
					Port:     uint16(servicePortHTTPSNodePort),
					URI:      service.Annotations[annotationLoadBalancerServiceHealthCheckURI],
					Interval: serviceHealthcheckInterval,
					Timeout:  serviceHealthcheckTimeout,
					Retries:  serviceHealthcheckRetries,
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
	expected.Services[0].Name = service.Annotations[annotationLoadBalancerServiceName]
	expected.Services[0].Description = service.Annotations[annotationLoadBalancerServiceDescription]
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
			&egoscale.NetworkLoadBalancer{Name: "lb", Description: "lb desc"},
			&egoscale.NetworkLoadBalancer{Name: "lb", Description: "lb desc"},
			require.False,
		},
		{
			"description updated",
			&egoscale.NetworkLoadBalancer{Name: "lb"},
			&egoscale.NetworkLoadBalancer{Name: "lb", Description: "lb desc"},
			require.True,
		},
		{
			"name updated",
			&egoscale.NetworkLoadBalancer{Description: "lb desc"},
			&egoscale.NetworkLoadBalancer{Name: "lb", Description: "lb desc"},
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
			&egoscale.NetworkLoadBalancerService{Name: "svc", Description: "svc desc"},
			&egoscale.NetworkLoadBalancerService{Name: "svc", Description: "svc desc"},
			require.False,
		},
		{
			"description updated",
			&egoscale.NetworkLoadBalancerService{Name: "svc"},
			&egoscale.NetworkLoadBalancerService{Name: "svc", Description: "svc desc"},
			require.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertion(t, isLoadBalancerServiceUpdated(tt.svcA, tt.svcB))
		})
	}
}
