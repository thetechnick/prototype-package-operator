package packages

import (
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type genericPackage interface {
	ClientObject() client.Object
	UpdatePhase()
	GetConditions() *[]metav1.Condition
	GetImage() string
	GetSource() interface{}
	SetStatusSourceHash(hash string)
	GetStatusSourceHash() string
}

var (
	_ genericPackage = (*GenericPackage)(nil)
	_ genericPackage = (*GenericClusterPackage)(nil)
)

type GenericPackage struct {
	packagesv1alpha1.Package
}

func (a *GenericPackage) ClientObject() client.Object {
	return &a.Package
}

func (a *GenericPackage) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.PackageProgressing,
	) {
		a.Status.Phase = packagesv1alpha1.PackagePhaseProgressing
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.PackageAvailable,
	) {
		a.Status.Phase = packagesv1alpha1.PackagePhaseAvailable
		return
	}

	a.Status.Phase = packagesv1alpha1.PackagePhaseNotReady
}

func (a *GenericPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericPackage) GetImage() string {
	return *a.Spec.Image
}

func (a *GenericPackage) GetSource() interface{} {
	return a.Spec.PackageSourceSpec
}

func (a *GenericPackage) SetStatusSourceHash(hash string) {
	a.Status.SourceHash = hash
}

func (a *GenericPackage) GetStatusSourceHash() string {
	return a.Status.SourceHash
}

type GenericClusterPackage struct {
	packagesv1alpha1.ClusterPackage
}

func (a *GenericClusterPackage) ClientObject() client.Object {
	return &a.ClusterPackage
}

func (a *GenericClusterPackage) UpdatePhase() {
	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.PackageProgressing,
	) {
		a.Status.Phase = packagesv1alpha1.PackagePhaseProgressing
		return
	}

	if meta.IsStatusConditionTrue(
		a.Status.Conditions,
		packagesv1alpha1.PackageAvailable,
	) {
		a.Status.Phase = packagesv1alpha1.PackagePhaseAvailable
		return
	}

	a.Status.Phase = packagesv1alpha1.PackagePhaseNotReady
}

func (a *GenericClusterPackage) GetConditions() *[]metav1.Condition {
	return &a.Status.Conditions
}

func (a *GenericClusterPackage) GetImage() string {
	return *a.Spec.Image
}

func (a *GenericClusterPackage) GetSource() interface{} {
	return a.Spec.PackageSourceSpec
}

func (a *GenericClusterPackage) SetStatusSourceHash(hash string) {
	a.Status.SourceHash = hash
}

func (a *GenericClusterPackage) GetStatusSourceHash() string {
	return a.Status.SourceHash
}

type genericObjectDeployment interface {
	ClientObject() client.Object
	GetPhases() []packagesv1alpha1.ObjectPhase
	SetPhases(phases []packagesv1alpha1.ObjectPhase)
	GetConditions() []metav1.Condition
}

var (
	_ genericObjectDeployment = (*GenericObjectDeployment)(nil)
	_ genericObjectDeployment = (*GenericClusterObjectDeployment)(nil)
)

type GenericObjectDeployment struct {
	packagesv1alpha1.ObjectDeployment
}

func (a *GenericObjectDeployment) ClientObject() client.Object {
	return &a.ObjectDeployment
}

func (a *GenericObjectDeployment) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericObjectDeployment) SetPhases(phases []packagesv1alpha1.ObjectPhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

type GenericClusterObjectDeployment struct {
	packagesv1alpha1.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) ClientObject() client.Object {
	return &a.ClusterObjectDeployment
}

func (a *GenericClusterObjectDeployment) GetPhases() []packagesv1alpha1.ObjectPhase {
	return a.Spec.Template.Spec.Phases
}

func (a *GenericClusterObjectDeployment) SetPhases(phases []packagesv1alpha1.ObjectPhase) {
	a.Spec.Template.Spec.Phases = phases
}

func (a *GenericClusterObjectDeployment) GetConditions() []metav1.Condition {
	return a.Status.Conditions
}

var (
	packageGVK        = packagesv1alpha1.GroupVersion.WithKind("Package")
	clusterPackageGVK = packagesv1alpha1.GroupVersion.WithKind("ClusterPackage")
)

func newPackage(scheme *runtime.Scheme) genericPackage {
	obj, err := scheme.New(packageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericPackage{Package: *obj.(*packagesv1alpha1.Package)}
}

func newClusterPackage(scheme *runtime.Scheme) genericPackage {
	obj, err := scheme.New(clusterPackageGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterPackage{ClusterPackage: *obj.(*packagesv1alpha1.ClusterPackage)}
}

var (
	objectDeploymentGVK        = packagesv1alpha1.GroupVersion.WithKind("ObjectDeployment")
	clusterObjectDeploymentGVK = packagesv1alpha1.GroupVersion.WithKind("ClusterObjectDeployment")
)

func newObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(objectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericObjectDeployment{ObjectDeployment: *obj.(*packagesv1alpha1.ObjectDeployment)}
}

func newClusterObjectDeployment(scheme *runtime.Scheme) genericObjectDeployment {
	obj, err := scheme.New(clusterObjectDeploymentGVK)
	if err != nil {
		panic(err)
	}

	return &GenericClusterObjectDeployment{ClusterObjectDeployment: *obj.(*packagesv1alpha1.ClusterObjectDeployment)}
}
