package packages

import (
	"context"

	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TeardownPhase(
	ctx context.Context,
	c client.Client,
	owner PausingClientObject,
	phase packagesv1alpha1.ObjectPhase,
) (cleanupDone bool, err error) {
	var (
		objectsToCleanup int
		cleanupCounter   int
	)
	for _, phaseObject := range phase.Objects {
		obj, err := UnstructuredFromObjectObject(&phaseObject)
		if err != nil {
			return false, err
		}
		obj.SetNamespace(owner.ClientObject().GetNamespace())

		if !owner.IsObjectPaused(obj) {
			objectsToCleanup++

			err := c.Delete(ctx, obj)
			if err != nil && errors.IsNotFound(err) {
				cleanupCounter++
				continue
			}
			if err != nil {
				return false, err
			}
		}
	}
	return cleanupCounter == objectsToCleanup, nil
}
