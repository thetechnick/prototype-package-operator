package objectdeployments

import (
	"context"

	"github.com/thetechnick/package-operator/internal/controllers/packages"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Computes the TemplateHash status property of ObjectDeployment objects.
type HashReconciler struct{}

func (r *HashReconciler) Reconcile(
	ctx context.Context, objectDeployment genericObjectDeployment,
) (ctrl.Result, error) {
	templateHash := packages.ComputeHash(
		objectDeployment.GetObjectSetTemplate(),
		objectDeployment.GetStatusCollisionCount())
	objectDeployment.SetStatusTemplateHash(templateHash)
	return ctrl.Result{}, nil
}
