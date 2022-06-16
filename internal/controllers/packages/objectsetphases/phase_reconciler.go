package objectsetphases

import (
	"context"
	"fmt"
	"strings"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type PhaseReconciler struct {
	phaseReconciler phaseReconciler
}

type phaseReconciler interface {
	Reconcile(
		ctx context.Context,
		owner packages.PausingClientObject,
		phase packagesv1alpha1.ObjectPhase,
		probe internalprobe.Interface,
	) (failedProbes []string, err error)
}

func (r *PhaseReconciler) Reconcile(
	ctx context.Context,
	objectSetPhase genericObjectSetPhase,
) (ctrl.Result, error) {
	probe := packages.ParseProbes(objectSetPhase.GetReadinessProbes())
	phase := objectSetPhase.GetPhase()

	var (
		failedProbes []string
		err          error
	)
	failedProbes, err = r.phaseReconciler.Reconcile(ctx, objectSetPhase, phase, probe)
	if err != nil {
		return ctrl.Result{}, err
	}

	if len(failedProbes) > 0 {
		meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.ObjectSetAvailable,
			Status:             metav1.ConditionFalse,
			Reason:             "ProbeFailure",
			Message:            fmt.Sprintf("Phase %q failed: %s", phase.Name, strings.Join(failedProbes, ", ")),
			ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
		})
		return ctrl.Result{}, nil
	}

	meta.SetStatusCondition(objectSetPhase.GetConditions(), metav1.Condition{
		Type:               packagesv1alpha1.ObjectSetAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSetPhase.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}
