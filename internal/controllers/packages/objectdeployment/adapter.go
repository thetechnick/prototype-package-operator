package objectdeployment

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
	GetPackageSetTemplate() packagesv1alpha1.PackageSetTemplate
	GetRevisionHistoryLimit() *int
	SetStatusCollisionCount(*int32)
	GetStatusCollisionCount() *int32
	GetStatusTemplateHash() string
	SetStatusTemplateHash(templateHash string)
	// GetPhases() []packagesv1alpha1.PackagePhase
	// IsArchived() bool
	// IsPaused() bool
	// IsObjectPaused(obj client.Object) bool
	// GetPausedFor() []packagesv1alpha1.PackagePausedObject
	// SetStatusPausedFor(pausedFor []packagesv1alpha1.PackagePausedObject)
	// GetDependencies() []packagesv1alpha1.PackageDependency
	// GetReadinessProbes() []packagesv1alpha1.PackageProbe
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	packagesv1alpha1.PackageDeployment
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
	return &a.PackageDeployment
}

func (a *GenericObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.PackageDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = packagesv1alpha1.PackageDeploymentPhaseProgressing
}

func (a *GenericObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericObjectDeployment) GetPackageSetTemplate() packagesv1alpha1.PackageSetTemplate {
	return a.Spec.Template
}

func (a *GenericObjectDeployment) SetStatusTemplateHash(templateHash string) {
	a.Status.TemplateHash = templateHash
}

func (a *GenericObjectDeployment) GetStatusTemplateHash() string {
	return a.Status.TemplateHash
}

type GenericClusterObjectDeployment struct {
	packagesv1alpha1.ClusterPackageDeployment
}

func (a *GenericClusterObjectDeployment) GetRevisionHistoryLimit() *int {
	return a.Spec.RevisionHistoryLimit
}

func (a *GenericClusterObjectDeployment) SetStatusCollisionCount(cc *int32) {
	a.Status.CollisionCount = cc
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterPackageDeployment
}

func (a *GenericClusterObjectDeployment) UpdatePhase() {
	availableCond := meta.FindStatusCondition(
		a.Status.Conditions,
		packagesv1alpha1.PackageDeploymentAvailable)

	if availableCond != nil {
		if availableCond.Status == metav1.ConditionTrue {
			a.Status.Phase = packagesv1alpha1.ClusterPackageDeploymentPhaseAvailable
			return
		}
		if availableCond.Status == metav1.ConditionFalse {
			a.Status.Phase = packagesv1alpha1.ClusterPackageDeploymentPhaseNotReady
			return
		}
	}
	a.Status.Phase = packagesv1alpha1.ClusterPackageDeploymentPhaseProgressing
}

func (a *GenericClusterObjectDeployment) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterObjectDeployment) GetSelector() metav1.LabelSelector {
	return a.Spec.Selector
}

func (a *GenericClusterObjectDeployment) GetPackageSetTemplate() packagesv1alpha1.PackageSetTemplate {
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
	GetTemplateSpec() packagesv1alpha1.PackageSetTemplateSpec
	SetTemplateSpec(templateSpec packagesv1alpha1.PackageSetTemplateSpec)
	GetPhases() []packagesv1alpha1.PackagePhase
	GetConditions() []metav1.Condition
	SetSpecPausedFor(pausedFor []packagesv1alpha1.PackagePausedObject)
	GetSpecPausedFor() []packagesv1alpha1.PackagePausedObject
	GetStatusPausedFor() []packagesv1alpha1.PackagePausedObject
	SetArchived()
}

type GenericObjectSet struct {
	packagesv1alpha1.PackageSet
}

func (a *GenericObjectSet) GetTemplateSpec() packagesv1alpha1.PackageSetTemplateSpec {
	return a.Spec.PackageSetTemplateSpec
}

func (a *GenericObjectSet) SetTemplateSpec(templateSpec packagesv1alpha1.PackageSetTemplateSpec) {
	a.Spec.PackageSetTemplateSpec = templateSpec
}

func (a *GenericObjectSet) ClientObject() client.Object {
	return &a.PackageSet
}

func (a *GenericObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericObjectSet) GetPhases() []packagesv1alpha1.PackagePhase {
	return a.Spec.Phases
}

func (a *GenericObjectSet) SetSpecPausedFor(
	pausedFor []packagesv1alpha1.PackagePausedObject) {
	a.Spec.PausedFor = pausedFor
}

func (a *GenericObjectSet) GetSpecPausedFor() []packagesv1alpha1.PackagePausedObject {
	return a.Spec.PausedFor
}

func (a *GenericObjectSet) GetStatusPausedFor() []packagesv1alpha1.PackagePausedObject {
	return a.Status.PausedFor
}

func (a *GenericObjectSet) SetArchived() {
	a.Spec.Archived = true
}

type GenericClusterObjectSet struct {
	packagesv1alpha1.ClusterPackageSet
}

func (a *GenericClusterObjectSet) SetArchived() {
	a.Spec.Archived = true
}

func (a *GenericClusterObjectSet) GetTemplateSpec() packagesv1alpha1.PackageSetTemplateSpec {
	return a.Spec.PackageSetTemplateSpec
}

func (a *GenericClusterObjectSet) SetTemplateSpec(templateSpec packagesv1alpha1.PackageSetTemplateSpec) {
	a.Spec.PackageSetTemplateSpec = templateSpec
}

func (a *GenericClusterObjectSet) ClientObject() client.Object {
	return &a.ClusterPackageSet
}

func (a *GenericClusterObjectSet) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

func (a *GenericClusterObjectSet) GetPhases() []packagesv1alpha1.PackagePhase {
	return a.Spec.Phases
}

func (a *GenericClusterObjectSet) SetSpecPausedFor(
	pausedFor []packagesv1alpha1.PackagePausedObject) {
	a.Spec.PausedFor = pausedFor
}

func (a *GenericClusterObjectSet) GetSpecPausedFor() []packagesv1alpha1.PackagePausedObject {
	return a.Spec.PausedFor
}

func (a *GenericClusterObjectSet) GetStatusPausedFor() []packagesv1alpha1.PackagePausedObject {
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
	packagesv1alpha1.PackageSetList
}

func (a *GenericObjectSetList) ClientObjectList() client.ObjectList {
	return &a.PackageSetList
}

func (a *GenericObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericObjectSet{
			PackageSet: a.Items[i],
		}
	}
	return out
}

type GenericClusterObjectSetList struct {
	packagesv1alpha1.ClusterPackageSetList
}

func (a *GenericClusterObjectSetList) ClientObjectList() client.ObjectList {
	return &a.ClusterPackageSetList
}

func (a *GenericClusterObjectSetList) GetItems() []genericObjectSet {
	out := make([]genericObjectSet, len(a.Items))
	for i := range a.Items {
		out[i] = &GenericClusterObjectSet{
			ClusterPackageSet: a.Items[i],
		}
	}
	return out
}
