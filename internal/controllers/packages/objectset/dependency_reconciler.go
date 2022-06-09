package objectset

import (
	"context"
	"fmt"
	"strings"
	"time"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const dependencyRequeueDuration = 30 * time.Second

type ObjectSetDependencyReconciler struct {
	client          client.Client
	discoveryClient discovery.DiscoveryInterface
}

func (r *ObjectSetDependencyReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet) (ctrl.Result, error) {
	dependencies := objectSet.GetDependencies()
	if len(dependencies) == 0 {
		return ctrl.Result{}, nil
	}

	_, apiResourceLists, err := r.discoveryClient.ServerGroupsAndResources()
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("discovering available APIs: %w", err)
	}

	var missingGKV []string
	for _, dependency := range dependencies {
		if dependency.KubernetesAPI == nil {
			continue
		}

		gvk := schema.GroupVersionKind{
			Group:   dependency.KubernetesAPI.Group,
			Version: dependency.KubernetesAPI.Version,
			Kind:    dependency.KubernetesAPI.Kind,
		}
		if !hasGVK(gvk, apiResourceLists) {
			missingGKV = append(missingGKV, gvk.String())
		}
	}

	if len(missingGKV) == 0 {
		return ctrl.Result{}, nil
	}

	conditions := objectSet.GetConditions()

	meta.SetStatusCondition(conditions, metav1.Condition{
		Type:               packagesv1alpha1.PackageSetAvailable,
		Status:             metav1.ConditionFalse,
		Reason:             "MissingDependency",
		Message:            fmt.Sprintf("Missing objects in kubernetes API: %s", strings.Join(missingGKV, ", ")),
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})

	// Retry later
	return ctrl.Result{
		RequeueAfter: dependencyRequeueDuration,
	}, nil
}

func hasGVK(gvk schema.GroupVersionKind, apiResourceLists []*metav1.APIResourceList) bool {
	for _, apiResourceList := range apiResourceLists {
		if apiResourceList == nil {
			continue
		}

		if gvk.GroupVersion().String() == apiResourceList.GroupVersion {
			for _, apiResource := range apiResourceList.APIResources {
				if gvk.Kind == apiResource.Kind {
					return true
				}
			}
		}
	}
	return false
}
