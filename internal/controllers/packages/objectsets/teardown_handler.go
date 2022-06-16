package objectsets

import (
	"context"
	"fmt"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"github.com/thetechnick/package-operator/internal/controllers"
	"github.com/thetechnick/package-operator/internal/controllers/packages"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TeardownHandler struct {
	client            client.Client
	dw                dynamicWatchFreer
	newObjectSetPhase func() genericObjectSetPhase
}

func NewTeardownHandler(
	c client.Client,
	dw dynamicWatchFreer,
	newObjectSetPhase func() genericObjectSetPhase,
) *TeardownHandler {
	return &TeardownHandler{
		client:            c,
		dw:                dw,
		newObjectSetPhase: newObjectSetPhase,
	}
}

func (h *TeardownHandler) Teardown(
	ctx context.Context, objectSet genericObjectSet,
) (done bool, err error) {
	log := controllers.LoggerFromContext(ctx)

	phases := objectSet.GetPhases()
	reverse(phases) // teardown in reverse order

	// "scale to zero"
	for _, phase := range phases {
		log.Info("cleanup", "phase", phase.Name)
		if cleanupDone, err := h.teardownPhase(ctx, objectSet, phase); err != nil {
			return false, fmt.Errorf("error archiving phase: %w", err)
		} else if !cleanupDone {
			return false, nil
		}
	}

	return true, nil
}

func (h *TeardownHandler) teardownPhase(
	ctx context.Context,
	objectSet genericObjectSet,
	phase packagesv1alpha1.ObjectPhase,
) (cleanupDone bool, err error) {
	if len(phase.Class) > 0 {
		return h.teardownRemotePhase(ctx, objectSet, phase)
	}
	return packages.TeardownPhase(ctx, h.client, objectSet, phase)
}

func (h *TeardownHandler) teardownRemotePhase(
	ctx context.Context,
	objectSet genericObjectSet,
	phase packagesv1alpha1.ObjectPhase,
) (cleanupDone bool, err error) {
	log := controllers.LoggerFromContext(ctx)

	defer log.Info("teardown of remote phase", "phase", phase.Name, "cleanupDone", cleanupDone)
	objectSetPhase := h.newObjectSetPhase()
	err = h.client.Get(ctx, client.ObjectKey{
		Name:      objectSet.ClientObject().GetName() + "-" + phase.Name,
		Namespace: objectSet.ClientObject().GetNamespace(),
	}, objectSetPhase.ClientObject())
	if err != nil && errors.IsNotFound(err) {
		// object is already gone -> nothing to cleanup
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// ensure PausedObject is up-to-date, _before_ we delete
	if !equality.Semantic.DeepEqual(
		objectSetPhase.GetStatusPausedFor(),
		objectSet.GetPausedFor(),
	) {
		// needs update/more wait time for ack
		objectSetPhase.SetSpecPausedFor(objectSet.GetPausedFor())
		if err := h.client.Update(ctx, objectSetPhase.ClientObject()); err != nil {
			return false, fmt.Errorf("updating ObjectSetPhase before archival: %w", err)
		}
		return false, nil
	}

	err = h.client.Delete(ctx, objectSetPhase.ClientObject())
	if err != nil && errors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("deleting ObjectSetPhase for archival: %w", err)
	}
	// ObjectSetPhase is not confirmed to be gone
	// wait an extra turn.
	return false, nil
}

// reverse the order of a slice
func reverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
