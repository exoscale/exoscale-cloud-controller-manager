package exoscale

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
)

type kubeServicePatcher struct {
	ctx      context.Context
	kclient  kubernetes.Interface
	current  *v1.Service
	modified *v1.Service
}

func newServicePatcher(ctx context.Context, kclient kubernetes.Interface, service *v1.Service) kubeServicePatcher {
	return kubeServicePatcher{
		ctx:      ctx,
		kclient:  kclient,
		current:  service.DeepCopy(),
		modified: service,
	}
}

// Patch submits a patch request for the Service to add some annotations
// unless the updated service reference contains the same set of annotations.
func (ksp *kubeServicePatcher) Patch() error {
	currentJSON, err := json.Marshal(ksp.current)
	if err != nil {
		return fmt.Errorf("failed to serialize current original object: %s", err)
	}

	modifiedJSON, err := json.Marshal(ksp.modified)
	if err != nil {
		return fmt.Errorf("failed to serialize modified updated object: %s", err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(currentJSON, modifiedJSON, v1.Service{})
	if err != nil {
		return fmt.Errorf("failed to create 2-way merge patch: %s", err)
	}

	if len(patch) == 0 || string(patch) == "{}" {
		return nil
	}

	_, err = ksp.kclient.CoreV1().Services(ksp.current.Namespace).Patch(ksp.ctx, ksp.current.Name, types.StrategicMergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("failed to patch service object %s/%s: %s", ksp.current.Namespace, ksp.current.Name, err)
	}

	return nil
}
