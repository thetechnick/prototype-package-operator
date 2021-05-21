package packagedeployment

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPackageSetsByRevision(t *testing.T) {
	packageSets := []packagesv1alpha1.PackageSet{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set-2",
				Annotations: map[string]string{
					packageSetRevisionAnnotation: "2",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set-1",
				Annotations: map[string]string{
					packageSetRevisionAnnotation: "1",
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "set-4",
				CreationTimestamp: metav1.Now(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "set-3",
				Annotations: map[string]string{
					packageSetRevisionAnnotation: "3",
				},
			},
		},
	}

	sort.Sort(packageSetsByRevision(packageSets))
	var names []string
	for _, packageSet := range packageSets {
		names = append(names, packageSet.Name)
	}

	assert.Equal(t, []string{"set-1", "set-2", "set-3", "set-4"}, names)
}
