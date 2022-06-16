package objectset

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
)

type genericObjectSet interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetPhases() []packagesv1alpha1.ObjectSetPhaseSpec
	IsArchived() bool
	IsPaused() bool
	IsObjectPaused(obj client.Object) bool
	GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	SetStatusPausedFor(pausedFor []packagesv1alpha1.ObjectSetPausedObject)
	GetDependencies() []packagesv1alpha1.ObjectSetDependency
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

func (a *GenericObjectSet) GetDependencies() []packagesv1alpha1.ObjectSetDependency {
	return a.Spec.Dependencies
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *GenericObjectSet) IsPaused() bool {
	return a.Spec.Paused
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
		if pausedObjectMatches(pausedObject, obj) {
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
		if availableCond.Reason == "MissingDependency" {
			a.Status.Phase = packagesv1alpha1.ObjectSetPhaseMissingDependency
			return
		}
	}

	a.Status.Phase = packagesv1alpha1.ObjectSetPhaseNotReady
}

func (a *GenericObjectSet) IsArchived() bool {
	return a.Spec.Archived
}

func (a *GenericObjectSet) GetPhases() []packagesv1alpha1.ObjectSetPhaseSpec {
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

func (a *GenericClusterObjectSet) GetDependencies() []packagesv1alpha1.ObjectSetDependency {
	return a.Spec.Dependencies
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
		if availableCond.Reason == "MissingDependency" {
			a.Status.Phase = packagesv1alpha1.ObjectSetPhaseMissingDependency
			return
		}
	}

	a.Status.Phase = packagesv1alpha1.ObjectSetPhaseNotReady
}

func (a *GenericClusterObjectSet) IsPaused() bool {
	return a.Spec.Paused
}

func (a *GenericClusterObjectSet) GetPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericClusterObjectSet) IsArchived() bool {
	return a.Spec.Archived
}

func (a *GenericClusterObjectSet) GetPhases() []packagesv1alpha1.ObjectSetPhaseSpec {
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
		if pausedObjectMatches(pausedObject, obj) {
			return true
		}
	}
	return false
}

func (a *GenericClusterObjectSet) SetStatusPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Status.PausedFor = pausedFor
}

func pausedObjectMatches(ppo packagesv1alpha1.ObjectSetPausedObject, obj client.Object) bool {
	gvk := obj.GetObjectKind().GroupVersionKind()
	if gvk.Group == ppo.Group &&
		gvk.Kind == ppo.Kind &&
		obj.GetName() == ppo.Name {
		return true
	}
	return false
}

func unstructuredFromObjectObject(packageObject *packagesv1alpha1.ObjectSetObject) (*unstructured.Unstructured, error) {
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal(packageObject.Object.Raw, obj); err != nil {
		return nil, fmt.Errorf("converting RawExtension into unstructured: %w", err)
	}
	return obj, nil
}
