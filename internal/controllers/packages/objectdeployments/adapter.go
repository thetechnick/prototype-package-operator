package objectdeployments

import (
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericObjectDeployment interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetSelector() metav1.LabelSelector
	GetObjectSetTemplate() packagesv1alpha1.ObjectSetTemplate
	GetRevisionHistoryLimit() *int
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	packagesv1alpha1.ObjectDeployment
}

func (a *GenericObjectDeployment) GetRevisionHistoryLimit() *int {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericObjectDeployment) GetObjectSetTemplate() packagesv1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

type GenericClusterObjectDeployment struct {
	packagesv1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) GetRevisionHistoryLimit() *int {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.ObjectDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = packagesv1alpha1.ObjectDeploymentPhaseProgressing
}

func (a *GenericClusterObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericClusterObjectDeployment) GetObjectSetTemplate() packagesv1alpha1.ObjectSetTemplate {
	return a.Spec.Template
}

func (a *GenericClusterObjectDeployment) GetStatusCollisionCount() *int32 {
	return a.Status.CollisionCount
}

func (a *GenericClusterObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericClusterObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

type genericObjectSet interface {
	ClientObject() client.Object
	GetTemplateSpec() packagesv1alpha1.ObjectSetTemplateSpec
	SetTemplateSpec(templateSpec packagesv1alpha1.ObjectSetTemplateSpec)
	GetPhases() []packagesv1alpha1.ObjectPhase
	GetConditions() []metav1.Condition
	SetSpecPausedFor(pausedFor []packagesv1alpha1.ObjectSetPausedObject)
	GetSpecPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject
	SetArchived()
}

type GenericObjectSet struct {
	packagesv1alpha1.ObjectSet
}

func (a *GenericObjectSet) GetTemplateSpec() packagesv1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *GenericObjectSet) SetTemplateSpec(templateSpec packagesv1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.ObjectSet
}

func (a *GenericObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectSet) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Phases
}

func (a *GenericObjectSet) SetSpecPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Spec.PausedFor = pausedFor
}

func (a *GenericObjectSet) GetSpecPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericObjectSet) GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Status.PausedFor
}

func (a *GenericObjectSet) SetArchived() {
	a.Spec.LifecycleState = packagesv1alpha1.ObjectSetLifecycleStateArchived
}

type GenericClusterObjectSet struct {
	packagesv1alpha1.ClusterObjectSet
}

func (a *GenericClusterObjectSet) SetArchived() {
	a.Spec.LifecycleState = packagesv1alpha1.ObjectSetLifecycleStateArchived
}

func (a *GenericClusterObjectSet) GetTemplateSpec() packagesv1alpha1.ObjectSetTemplateSpec {
	return a.Spec.ObjectSetTemplateSpec
}

func (a *GenericClusterObjectSet) SetTemplateSpec(templateSpec packagesv1alpha1.ObjectSetTemplateSpec) {
	a.Spec.ObjectSetTemplateSpec = templateSpec
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterObjectSet
}

func (a *GenericClusterObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectSet) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) SetSpecPausedFor(
	pausedFor []packagesv1alpha1.ObjectSetPausedObject) {
	a.Spec.PausedFor = pausedFor
}

func (a *GenericClusterObjectSet) GetSpecPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Spec.PausedFor
}

func (a *GenericClusterObjectSet) GetStatusPausedFor() []packagesv1alpha1.ObjectSetPausedObject {
	return a.Status.PausedFor
}

type genericObjectSetList interface {
	ClientObjectList() client.ObjectList
	GetItems() []genericObjectSet
}

var (
	_ genericObjectSetList = (*GenericObjectSetList)(nil)
	_ genericObjectSetList = (*GenericClusterObjectSetList)(nil)
)

type GenericObjectSetList struct {
	packagesv1alpha1.ObjectSetList
}

func (a *GenericObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ObjectSetList
}

func (a *GenericObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericObjectSet{
			ObjectSet: a.Items[i],
		}
	}
	return out
}

type GenericClusterObjectSetList struct {
	packagesv1alpha1.ClusterObjectSetList
}

func (a *GenericClusterObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ClusterObjectSetList
}

func (a *GenericClusterObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericClusterObjectSet{
			ClusterObjectSet: a.Items[i],
		}
	}
	return out
}
