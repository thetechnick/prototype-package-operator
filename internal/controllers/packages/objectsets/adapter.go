package objectsets

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
)

type genericObjectSet interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetPhases() []packagesv1alpha1.ObjectPhase
	IsArchived() bool
	IsPaused() bool
	IsObjectPaused(obj client.Object) bool
	GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	SetStatusPausedFor(pausedFor []packagesv1alpha1.ObjectSetPausedObject)
	GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe
}

var (
	_ genericObjectSet = (*GenericObjectSet)(nil)
	_ genericObjectSet = (*GenericClusterObjectSet)(nil)
)

type GenericObjectSet struct {
	packagesv1alpha1.ObjectSet
}

func (a *GenericObjectSet) GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe {
	return a.Spec.ReadinessProbes
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *GenericObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == packagesv1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericObjectSet) GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericObjectSet) SetStatusPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Status.PausedFor = pausedFor
}

func (a *GenericObjectSet) IsObjectPaused(obj client.Object) bool {
	if a.IsPaused() {
		return true
	}
	for _, pausedObject := range a.GetPausedFor() {
		if packages.PausedObjectMatches(pausedObject, obj) {
			return true
		}
	}
	return false
}

func (a *GenericObjectSet) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.ObjectSetArchived,
	) {
		a.Status.Phase = packagesv1alpha1.ObjectSetPhaseArchived
		return
	}

	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.ObjectSetAvailable,
	)
	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.ObjectSetPhaseAvailable
			return
		}
	}

	a.Status.Phase = packagesv1alpha1.ObjectSetPhaseNotReady
}

func (a *GenericObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == packagesv1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericObjectSet) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Phases
}

func (a *GenericObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

type GenericClusterObjectSet struct {
	packagesv1alpha1.ClusterObjectSet
}

func (a *GenericClusterObjectSet) GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe {
	return a.Spec.ReadinessProbes
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *GenericClusterObjectSet) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.ObjectSetArchived,
	) {
		a.Status.Phase = packagesv1alpha1.ObjectSetPhaseArchived
		return
	}

	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.ObjectSetAvailable,
	)
	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.ObjectSetPhaseAvailable
			return
		}
	}

	a.Status.Phase = packagesv1alpha1.ObjectSetPhaseNotReady
}

func (a *GenericClusterObjectSet) IsPaused() bool {
	return a.Spec.LifecycleState == packagesv1alpha1.ObjectSetLifecycleStatePaused
}

func (a *GenericClusterObjectSet) GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericClusterObjectSet) IsArchived() bool {
	return a.Spec.LifecycleState == packagesv1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericClusterObjectSet) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectSet) IsObjectPaused(obj client.Object) bool {
	if a.IsPaused() {
		return true
	}
	for _, pausedObject := range a.GetPausedFor() {
		if packages.PausedObjectMatches(pausedObject, obj) {
			return true
		}
	}
	return false
}

func (a *GenericClusterObjectSet) SetStatusPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Status.PausedFor = pausedFor
}

type genericObjectSetPhase interface {
	ClientObject() client.Object
	GetConditions() []metav1.Condition
	SetPhase(phase packagesv1alpha1.ObjectPhase)
	SetReadinessProbes(probes []packagesv1alpha1.ObjectSetProbe)
	GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	SetSpecPausedFor(pausedFor []packagesv1alpha1.ObjectSetPausedObject)
}

var (
	_ genericObjectSetPhase = (*GenericObjectSetPhase)(nil)
	_ genericObjectSetPhase = (*GenericClusterObjectSetPhase)(nil)
)

type GenericObjectSetPhase struct {
	packagesv1alpha1.ObjectSetPhase
}

func (a *GenericObjectSetPhase) ClientObject() client.Object {
	return &a.ObjectSetPhase
}

func (a *GenericObjectSetPhase) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectSetPhase) SetPhase(phase packagesv1alpha1.ObjectPhase) {
	a.Spec.ObjectPhase = phase
}

func (a *GenericObjectSetPhase) SetReadinessProbes(probes []packagesv1alpha1.ObjectSetProbe) {
	a.Spec.ReadinessProbes = probes
}

func (a *GenericObjectSetPhase) SetSpecPausedFor(paused []packagesv1alpha1.ObjectSetPausedObject) {
	a.Spec.PausedFor = paused
}

func (a *GenericObjectSetPhase) GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Status.PausedFor
}

type GenericClusterObjectSetPhase struct {
	packagesv1alpha1.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) ClientObject() client.Object {
	return &a.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectSetPhase) SetPhase(phase packagesv1alpha1.ObjectPhase) {
	a.Spec.ObjectPhase = phase
}

func (a *GenericClusterObjectSetPhase) SetReadinessProbes(probes []packagesv1alpha1.ObjectSetProbe) {
	a.Spec.ReadinessProbes = probes
}

func (a *GenericClusterObjectSetPhase) SetSpecPausedFor(paused []packagesv1alpha1.ObjectSetPausedObject) {
	a.Spec.PausedFor = paused
}

func (a *GenericClusterObjectSetPhase) GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Status.PausedFor
}
