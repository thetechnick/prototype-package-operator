package objectsets

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	internalprobe "github.com/thetechnick/package-operator/internal/probe"
)

type ObjectSetPhaseReconciler struct {
	client            client.Client
	scheme            *runtime.Scheme
	dw                dynamicObjectWatcher
	newObjectSetPhase func() genericObjectSetPhase
	phaseReconciler   phaseReconciler
}

type phaseReconciler interface {
	Reconcile(
		ctx context.Context,
		owner packages.PausingClientObject,
		phase packagesv1alpha1.ObjectPhase,
		probe internalprobe.Interface,
	) (failedProbes []string, err error)
}

type dynamicObjectWatcher interface {
	Watch(owner client.Object, obj runtime.Object) error
}

func (r *ObjectSetPhaseReconciler) Reconcile(
	ctx context.Context, objectSet genericObjectSet,
) (ctrl.Result, error) {
	if objectSet.IsArchived() {
		return ctrl.Result{}, nil
	}

	probe := packages.ParseProbes(objectSet.GetReadinessProbes())
	phases := objectSet.GetPhases()
	for _, phase := range phases {
		var (
			failedProbes []string
			err          error
		)
		if len(phase.Class) > 0 {
			failedProbes, err = r.reconcileRemotePhase(
				ctx, objectSet, phase)
		} else {
			failedProbes, err = r.reconcileLocalPhase(
				ctx, objectSet, phase, probe)
		}
		if err != nil {
			return ctrl.Result{}, err
		}

		if len(failedProbes) > 0 {
			meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
				Type:               packagesv1alpha1.ObjectSetAvailable,
				Status:             metav1.ConditionFalse,
				Reason:             "ProbeFailure",
				Message:            fmt.Sprintf("Phase %q failed: %s", phase.Name, strings.Join(failedProbes, ", ")),
				ObservedGeneration: objectSet.ClientObject().GetGeneration(),
			})
			return ctrl.Result{}, nil
		}
	}

	if !meta.IsStatusConditionTrue(*objectSet.GetConditions(), packagesv1alpha1.ObjectSetSucceeded) {
		meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
			Type:               packagesv1alpha1.ObjectSetSucceeded,
			Status:             metav1.ConditionTrue,
			Reason:             "AvailableOnce",
			Message:            "Object was available once and passed all probes.",
			ObservedGeneration: objectSet.ClientObject().GetGeneration(),
		})
	}

	meta.RemoveStatusCondition(objectSet.GetConditions(), packagesv1alpha1.ObjectSetArchived)
	meta.SetStatusCondition(objectSet.GetConditions(), metav1.Condition{
		Type:               packagesv1alpha1.ObjectSetAvailable,
		Status:             metav1.ConditionTrue,
		Reason:             "Available",
		Message:            "Object is available and passes all probes.",
		ObservedGeneration: objectSet.ClientObject().GetGeneration(),
	})

	return ctrl.Result{}, nil
}

const noStatusProbeFailure = "no status reported"

// Reconciles the Phase via an ObjectSetPhase object,
// delegating the task to an auxiliary controller.
func (r *ObjectSetPhaseReconciler) reconcileRemotePhase(
	ctx context.Context,
	objectSet genericObjectSet,
	phase packagesv1alpha1.ObjectPhase,
) (failedProbes []string, err error) {
	os := objectSet.ClientObject()
	newObjectSetPhase := r.newObjectSetPhase()
	new := newObjectSetPhase.ClientObject()
	new.SetName(os.GetName() + "-" + phase.Name)
	new.SetNamespace(os.GetNamespace())
	new.SetAnnotations(os.GetAnnotations())
	new.SetLabels(os.GetLabels())

	newObjectSetPhase.SetPhase(phase)
	newObjectSetPhase.SetReadinessProbes(objectSet.GetReadinessProbes())

	if err := controllerutil.SetControllerReference(
		os, new, r.scheme); err != nil {
		return nil, err
	}

	existingObjectSetPhase := r.newObjectSetPhase()
	if err := r.client.Get(
		ctx, client.ObjectKeyFromObject(new),
		existingObjectSetPhase.ClientObject(),
	); err != nil && !errors.IsNotFound(err) {
		return nil,
			fmt.Errorf("getting existing ObjectSetPhase: %w", err)
	} else if errors.IsNotFound(err) {
		if err := r.client.Create(
			ctx, newObjectSetPhase.ClientObject()); err != nil {
			return nil,
				fmt.Errorf("creating new ObjectSetPhase: %w", err)
		}
		// wait for requeue
		return nil, nil
	}

	// ObjectSetPhase already exists
	// -> check status
	availableCond := meta.FindStatusCondition(
		existingObjectSetPhase.GetConditions(),
		packagesv1alpha1.ObjectSetAvailable,
	)
	if availableCond == nil ||
		availableCond.ObservedGeneration !=
			existingObjectSetPhase.ClientObject().GetGeneration() {
		// no status reported, wait longer
		return []string{
			noStatusProbeFailure,
		}, nil
	}
	if availableCond.Status == metav1.ConditionTrue {
		// Remote Phase is Available!
		return nil, nil
	}

	// Remote Phase is not Available!
	return []string{
		availableCond.Message,
	}, nil
}

// Reconciles the Phase directly in-process
func (r *ObjectSetPhaseReconciler) reconcileLocalPhase(
	ctx context.Context,
	objectSet genericObjectSet,
	phase packagesv1alpha1.ObjectPhase,
	probe internalprobe.Interface,
) ([]string, error) {
	return r.phaseReconciler.Reconcile(ctx, objectSet, phase, probe)
}
