package packages

import (
	"context"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type objectDeploymentReconciler struct {
	client              client.Client
	scheme              *runtime.Scheme
	newObjectDeployment objectDeploymentFactory
}

func newObjectDeploymentReconciler(
	c client.Client,
	scheme *runtime.Scheme,
	newObjectDeployment objectDeploymentFactory,
) *objectDeploymentReconciler {
	return &objectDeploymentReconciler{
		client:              c,
		scheme:              scheme,
		newObjectDeployment: newObjectDeployment,
	}
}

func (c *objectDeploymentReconciler) Reconcile(
	ctx context.Context, packageObj genericPackage,
) (ctrl.Result, error) {
	return ctrl.Result{}, c.reconcileDeployment(ctx, packageObj)
}

func (c *objectDeploymentReconciler) reconcileDeployment(
	ctx context.Context, packageObj genericPackage,
) error {
	deploy := c.newObjectDeployment(c.scheme)
	err := c.client.Get(
		ctx, client.ObjectKeyFromObject(packageObj.ClientObject()),
		deploy.ClientObject(),
	)
	if err != nil {
		// no status to propagate when there is no deployment object
		return client.IgnoreNotFound(err)
	}

	// Copy conditions from the ObjectDeployment
	if deployAvailableCond := meta.FindStatusCondition(
		deploy.GetConditions(), packagesv1alpha1.ObjectDeploymentAvailable,
	); deployAvailableCond != nil &&
		deployAvailableCond.ObservedGeneration == deploy.ClientObject().GetGeneration() {
		packageAvailableCond := deployAvailableCond.DeepCopy()
		packageAvailableCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageAvailableCond)
	}

	if deployProgressingCond := meta.FindStatusCondition(
		deploy.GetConditions(), packagesv1alpha1.ObjectDeploymentAvailable,
	); deployProgressingCond != nil &&
		deployProgressingCond.ObservedGeneration == deploy.ClientObject().GetGeneration() {
		packageProgressingCond := deployProgressingCond.DeepCopy()
		packageProgressingCond.ObservedGeneration = packageObj.ClientObject().GetGeneration()

		meta.SetStatusCondition(packageObj.GetConditions(), *packageProgressingCond)
	}

	return nil
}
