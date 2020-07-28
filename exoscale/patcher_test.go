package exoscale

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

const testServiceID = "009e9cb6-147e-4a33-ab93-7651f246cb5c"

func newFakeService() *v1.Service {
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "service-name",
			Namespace:   metav1.NamespaceDefault,
			Annotations: map[string]string{},
		},
	}
}

// TestNewServicePatcher tests that the kubeServicePatcher object returned correctly snapshots the v1.Service object
// internally.
func TestNewServicePatcher(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	service := newFakeService()
	patcher := newServicePatcher(ctx, clientset, service)

	require.Equal(t, patcher.current, patcher.modified, "service values should not differ")
	service.ObjectMeta.Annotations[annotationLoadBalancerID] = testServiceID
	require.NotEqual(t, patcher.current, patcher.modified, "service values should differ")
}

// TestKubeServicePatcherPatch tests that the kubeServicePatcher correctly patches the Service if annotations are
// added internally.
func TestKubeServicePatcherPatch(t *testing.T) {
	ctx := context.Background()
	clientset := fake.NewSimpleClientset()

	service := newFakeService()
	serviceAdded, err := clientset.CoreV1().Services(metav1.NamespaceDefault).Create(ctx, service, metav1.CreateOptions{})
	require.NoError(t, err)

	svcID := serviceAdded.ObjectMeta.Annotations[annotationLoadBalancerID]
	require.Empty(t, svcID)

	patcher := newServicePatcher(ctx, clientset, service)
	service.ObjectMeta.Annotations[annotationLoadBalancerID] = testServiceID
	err = patcher.Patch()
	require.NoError(t, err)

	serviceFinal, err := clientset.CoreV1().Services(metav1.NamespaceDefault).Get(ctx, service.Name, metav1.GetOptions{})
	require.NoError(t, err)

	svcID = serviceFinal.ObjectMeta.Annotations[annotationLoadBalancerID]
	require.Equal(t, svcID, testServiceID)
}
