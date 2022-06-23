package objectsetphases

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
)

type genericObjectSetPhase interface {
	ClientObject() client.Object
	GetConditions() *[]metav1.Condition
	SetPhase(phase packagesv1alpha1.ObjectPhase)
	GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	SetStatusPausedFor(pausedFor []packagesv1alpha1.ObjectSetPausedObject)
	GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe
	GetPhase() packagesv1alpha1.ObjectPhase
	IsPaused() bool
	GetClass() string
	IsObjectPaused(obj client.Object) bool
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

func (a *GenericObjectSetPhase) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectSetPhase) SetPhase(phase packagesv1alpha1.ObjectPhase) {
	a.Spec.ObjectPhase = phase
}

func (a *GenericObjectSetPhase) GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericObjectSetPhase) SetStatusPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Status.PausedFor = pausedFor
}

func (a *GenericObjectSetPhase) GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe {
	return a.Spec.ReadinessProbes
}

func (a *GenericObjectSetPhase) GetPhase() packagesv1alpha1.ObjectPhase {
	return a.Spec.ObjectPhase
}

func (a *GenericObjectSetPhase) IsObjectPaused(obj client.Object) bool {
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

func (a *GenericObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericObjectSetPhase) GetClass() string {
	return a.Spec.Class
}

type GenericClusterObjectSetPhase struct {
	packagesv1alpha1.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) ClientObject() client.Object {
	return &a.ClusterObjectSetPhase
}

func (a *GenericClusterObjectSetPhase) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectSetPhase) SetPhase(phase packagesv1alpha1.ObjectPhase) {
	a.Spec.ObjectPhase = phase
}

func (a *GenericClusterObjectSetPhase) GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericClusterObjectSetPhase) SetStatusPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Status.PausedFor = pausedFor
}

func (a *GenericClusterObjectSetPhase) GetReadinessProbes() []packagesv1alpha1.ObjectSetProbe {
	return a.Spec.ReadinessProbes
}

func (a *GenericClusterObjectSetPhase) GetPhase() packagesv1alpha1.ObjectPhase {
	return a.Spec.ObjectPhase
}

func (a *GenericClusterObjectSetPhase) IsObjectPaused(obj client.Object) bool {
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

func (a *GenericClusterObjectSetPhase) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericClusterObjectSetPhase) GetClass() string {
	return a.Spec.Class
}
