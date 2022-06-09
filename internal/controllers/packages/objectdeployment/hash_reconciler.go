package objectdeployment

import (
	"context"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"

	"github.com/davecgh/go-spew/spew"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Computes the TemplateHash status property of ObjectDeployment objects.
type HashReconciler struct{}

func (r *HashReconciler) Reconcile(
	ctx context.Context, objectDeployment genericObjectDeployment,
) (ctrl.Result, error) {
	templateHash := computeHash(
		objectDeployment.GetPackageSetTemplate(),
		objectDeployment.GetStatusCollisionCount())
	objectDeployment.SetStatusTemplateHash(templateHash)
	return ctrl.Result{}, nil
}

// computeHash returns a hash value calculated from pod template and
// a collisionCount to avoid hash collision. The hash will be safe encoded to
// avoid bad words.
func computeHash(template packagesv1alpha1.PackageSetTemplate, collisionCount *int32) string {
	hasher := fnv.New32a()
	deepHashObject(hasher, template)

	// Add collisionCount in the hash if it exists.
	if collisionCount != nil {
		collisionCountBytes := make([]byte, 8)
		binary.LittleEndian.PutUint32(
			collisionCountBytes, uint32(*collisionCount))
		hasher.Write(collisionCountBytes)
	}

	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}

// deepHashObject writes specified object to hash using the spew library
// which follows pointers and prints actual values of the nested objects
// ensuring the hash does not change when a pointer changes.
func deepHashObject(hasher hash.Hash, objectToWrite interface{}) {
	hasher.Reset()
	printer := spew.ConfigState{
		Indent:         " ",
		SortKeys:       true,
		DisableMethods: true,
		SpewKeys:       true,
	}
	printer.Fprintf(hasher, "%#v", objectToWrite)
}
