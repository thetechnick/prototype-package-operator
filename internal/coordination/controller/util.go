package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
	"github.com/thetechnick/package-operator/internal/dynamicwatcher"
)

const (
	cacheFinalizer = "coordination.thetechnick.ninja/cache"
)

// ensures the cache finalizer is set on the given object
func ensureCacheFinalizer(
	ctx context.Context, c client.Client, obj client.Object) error {
	if !controllerutil.ContainsFinalizer(
		obj, cacheFinalizer) {
		controllerutil.AddFinalizer(obj, cacheFinalizer)
		if err := c.Update(ctx, obj); err != nil {
			return fmt.Errorf("adding finalizer: %w", err)
		}
	}
	return nil
}

// builds unstrucutred objects from a TargetAPI object.
func unstructuredFromTargetAPI(targetAPI coordinationv1alpha1.TargetAPI) (
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

// handle deletion by removing the finalizer and freeing watchers.
func handleDeletion(
	ctx context.Context, c client.Client, dw *dynamicwatcher.DynamicWatcher, obj client.Object,
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

// given a list of objects this function will group all objects with the same label value.
// the return slice is garanteed to be of the same size as the amount of values given to the function.
func groupByLabelValues(in []unstructured.Unstructured, labelKey string, values ...string) [][]unstructured.Unstructured {
	out := make([][]unstructured.Unstructured, len(values))
	for _, obj := range in {
		if obj.GetLabels() == nil {
			continue
		}
		if len(obj.GetLabels()[labelKey]) == 0 {
			continue
		}

		for i, v := range values {
			if obj.GetLabels()[labelKey] == v {
				out[i] = append(out[i], obj)
			}
		}
	}
	return out
}
