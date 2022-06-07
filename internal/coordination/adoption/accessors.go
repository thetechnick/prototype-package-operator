package adoption

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
)

type strategyType string

const (
	staticStrategy     strategyType = "Static"
	roundRobinStrategy strategyType = "RoundRobin"
)

func getStrategyType(adoption client.Object) strategyType {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return strategyType(o.Spec.Strategy.Type)
	case *coordinationv1alpha1.ClusterAdoption:
		return strategyType(o.Spec.Strategy.Type)
	}
	return ""
}

func setStatus(adoption client.Object) {
	var conds *[]metav1.Condition
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		o.Status.ObservedGeneration = o.Generation
		o.Status.Phase = coordinationv1alpha1.AdoptionPhaseActive

	case *coordinationv1alpha1.ClusterAdoption:
		o.Status.ObservedGeneration = o.Generation
		o.Status.Phase = coordinationv1alpha1.ClusterAdoptionPhaseActive
	}

	meta.SetStatusCondition(conds, metav1.Condition{
		Type:               coordinationv1alpha1.AdoptionActive,
		Status:             metav1.ConditionTrue,
		Reason:             "Setup",
		Message:            "Controller is setup and adding labels.",
		ObservedGeneration: adoption.GetGeneration(),
	})
}

func getTargetAPI(adoption client.Object) coordinationv1alpha1.TargetAPI {
	switch o := adoption.(type) {
	case *coordinationv1alpha1.Adoption:
		return o.Spec.TargetAPI
	case *coordinationv1alpha1.ClusterAdoption:
		return o.Spec.TargetAPI
	}
	return coordinationv1alpha1.TargetAPI{}
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
