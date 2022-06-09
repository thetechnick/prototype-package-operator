package coordination

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
)

const (
	cacheFinalizer = "coordination.thetechnick.ninja/cache"
)

func EnsureCommonFinalizer(
	ctx context.Context, obj client.Object,
	c client.Client,
) error {
	if !controllerutil.ContainsFinalizer(
		obj, cacheFinalizer) {
		controllerutil.AddFinalizer(obj, cacheFinalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

type dynamicWatchFreer interface {
	Free(obj client.Object) error
}

func HandleCommonDeletion(
	ctx context.Context, obj client.Object,
	c client.Client, dw dynamicWatchFreer,
) error {
	if controllerutil.ContainsFinalizer(obj, cacheFinalizer) {
		controllerutil.RemoveFinalizer(obj, cacheFinalizer)

		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("removing finalizer: %w", err)
		}
	}

	if err := dw.Free(obj); err != nil {
		return fmt.Errorf("free cache: %w", err)
	}
	return nil
}

// builds unstrucutred objects from a TargetAPI object.
func UnstructuredFromTargetAPI(targetAPI coordinationv1alpha1.TargetAPI) (
	gvk schema.GroupVersionKind,
	objType *unstructured.Unstructured,
	objListType *unstructured.UnstructuredList,
) {
	gvk = schema.GroupVersionKind{
		Group:   targetAPI.Group,
		Version: targetAPI.Version,
		Kind:    targetAPI.Kind,
	}

	objType = &unstructured.Unstructured{}
	objType.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   targetAPI.Group,
		Version: targetAPI.Version,
		Kind:    targetAPI.Kind,
	})

	objListType = &unstructured.UnstructuredList{}
	objListType.SetGroupVersionKind(gvk)
	objListType.SetKind(gvk.Kind + "List")
	return
}
