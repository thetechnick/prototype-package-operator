package packages

import (
	"context"

	"github.com/thetechnick/package-operator/internal/controllers/packages"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Computes the SourceHash status property of Package objects.
type hashReconciler struct{}

func newHashReconciler() *hashReconciler {
	return &hashReconciler{}
}

func (r *hashReconciler) Reconcile(
	ctx context.Context, packageObj genericPackage,
) (ctrl.Result, error) {
	templateHash := packages.ComputeHash(
		packageObj.GetSource(),
		nil, // can't collide, because Package:Job is 1:1
	)
	packageObj.SetStatusSourceHash(templateHash)
	return ctrl.Result{}, nil
}
