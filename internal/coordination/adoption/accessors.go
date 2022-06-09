package adoption

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	panic("invalid Adoption object")
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

	default:
		panic("invalid Adoption object")
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
	panic("invalid Adoption object")
}
