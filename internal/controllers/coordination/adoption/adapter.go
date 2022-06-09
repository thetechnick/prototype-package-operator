package adoption

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	coordinationv1alpha1 "github.com/thetechnick/package-operator/apis/coordination/v1alpha1"
)

type genericStrategyType string

const (
	genericStrategyStatic     genericStrategyType = "Static"
	genericStrategyRoundRobin genericStrategyType = "RoundRobin"
)

type genericAdoption interface {
	ClientObject() client.Object
	UpdatePhase()
	GetStrategyType() genericStrategyType
	GetStaticStrategy() *coordinationv1alpha1.AdoptionStrategyStaticSpec
	GetRoundRobinStrategy() *coordinationv1alpha1.AdoptionStrategyRoundRobinSpec
	GetTargetAPI() coordinationv1alpha1.TargetAPI
	GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus
	SetRoundRobinStatus(rr *coordinationv1alpha1.AdoptionRoundRobinStatus)
	GetConditions() *[]metav1.Condition
}

var (
	_ genericAdoption = (*GenericAdoption)(nil)
	_ genericAdoption = (*GenericClusterAdoption)(nil)
)

type GenericAdoption struct {
	coordinationv1alpha1.Adoption
}

func (a *GenericAdoption) ClientObject() client.Object {
	return &a.Adoption
}

func (a *GenericAdoption) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.AdoptionActive) {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhaseActive
	} else {
		a.Status.Phase = coordinationv1alpha1.AdoptionPhasePending
	}
}

func (a *GenericAdoption) GetStrategyType() genericStrategyType {
	return genericStrategyType(a.Spec.Strategy.Type)
}

func (a *GenericAdoption) GetStaticStrategy() *coordinationv1alpha1.AdoptionStrategyStaticSpec {
	return a.Spec.Strategy.Static
}

func (a *GenericAdoption) GetRoundRobinStrategy() *coordinationv1alpha1.AdoptionStrategyRoundRobinSpec {
	return a.Spec.Strategy.RoundRobin
}

func (a *GenericAdoption) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericAdoption) SetRoundRobinStatus(
	rr *coordinationv1alpha1.AdoptionRoundRobinStatus) {
	a.Status.RoundRobin = rr
}

func (a *GenericAdoption) GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus {
	return a.Status.RoundRobin
}

func (a *GenericAdoption) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

type GenericClusterAdoption struct {
	coordinationv1alpha1.ClusterAdoption
}

func (a *GenericClusterAdoption) ClientObject() client.Object {
	return &a.ClusterAdoption
}

func (a *GenericClusterAdoption) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions, coordinationv1alpha1.AdoptionActive) {
		a.Status.Phase = coordinationv1alpha1.ClusterAdoptionPhaseActive
	} else {
		a.Status.Phase = coordinationv1alpha1.ClusterAdoptionPhasePending
	}
}

func (a *GenericClusterAdoption) GetStrategyType() genericStrategyType {
	return genericStrategyType(a.Spec.Strategy.Type)
}

func (a *GenericClusterAdoption) GetStaticStrategy() *coordinationv1alpha1.AdoptionStrategyStaticSpec {
	return a.Spec.Strategy.Static
}

func (a *GenericClusterAdoption) GetRoundRobinStrategy() *coordinationv1alpha1.AdoptionStrategyRoundRobinSpec {
	return a.Spec.Strategy.RoundRobin
}

func (a *GenericClusterAdoption) GetTargetAPI() coordinationv1alpha1.TargetAPI {
	return a.Spec.TargetAPI
}

func (a *GenericClusterAdoption) SetRoundRobinStatus(
	rr *coordinationv1alpha1.AdoptionRoundRobinStatus) {
	a.Status.RoundRobin = rr
}

func (a *GenericClusterAdoption) GetRoundRobinStatus() *coordinationv1alpha1.AdoptionRoundRobinStatus {
	return a.Status.RoundRobin
}

func (a *GenericClusterAdoption) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}
