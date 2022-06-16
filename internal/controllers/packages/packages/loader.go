package packages

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"io/ioutil"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/go-logr/logr"
	packagesv1alpha1 "github.com/thetechnick/package-operator/apis/packages/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

const phaseAnnotation = "packages.thetechnick.ninja/phase"

type packageLoaderBuilder struct {
	log                 logr.Logger
	scheme              *runtime.Scheme
	newObjectDeployment objectDeploymentFactory
}

func newPackageLoaderBuilder(log logr.Logger, scheme *runtime.Scheme) *packageLoaderBuilder {
	return newGenericPackageLoaderBuilder(log, scheme, newObjectDeployment)
}

func newClusterPackageLoaderBuilder(log logr.Logger, scheme *runtime.Scheme) *packageLoaderBuilder {
	return newGenericPackageLoaderBuilder(log, scheme, newClusterObjectDeployment)
}

func newGenericPackageLoaderBuilder(log logr.Logger, scheme *runtime.Scheme, newObjectDeployment objectDeploymentFactory) *packageLoaderBuilder {
	return &packageLoaderBuilder{
		log:                 log,
		scheme:              scheme,
		newObjectDeployment: newObjectDeployment,
	}
}

func (b *packageLoaderBuilder) Load(path string, context map[string]interface{}) (genericObjectDeployment, error) {
	return (&packageLoader{
		log:                 b.log,
		scheme:              b.scheme,
		newObjectDeployment: b.newObjectDeployment,

		path:    path,
		context: context,
	}).Load()
}

type packageLoader struct {
	log                 logr.Logger
	scheme              *runtime.Scheme
	newObjectDeployment objectDeploymentFactory

	path    string
	context map[string]interface{}

	objectDeployment *unstructured.Unstructured
	phaseObjs        map[string][]unstructured.Unstructured
}

func (l *packageLoader) Load() (genericObjectDeployment, error) {
	l.phaseObjs = map[string][]unstructured.Unstructured{}
	l.objectDeployment = nil

	err := filepath.WalkDir(l.path, l.walk)
	if err != nil {
		return nil, fmt.Errorf("walking directory structure: %w", err)
	}

	if l.objectDeployment == nil {
		return nil, fmt.Errorf(
			"package contains no (Cluster)ObjectDeployment: %w", err)
	}

	odJson, err := json.Marshal(l.objectDeployment)
	if err != nil {
		return nil, fmt.Errorf("marshalling ObjectDeployment: %w", err)
	}

	od := l.newObjectDeployment(l.scheme)
	if err := json.Unmarshal(odJson, od); err != nil {
		return nil, fmt.Errorf("unmarshal ObjectDeployment: %w", err)
	}

	phases := od.GetPhases()
	knownPhases := map[string]struct{}{}
	for i := range phases {
		phase := phases[i]
		knownPhases[phase.Name] = struct{}{}

		if len(l.phaseObjs[phase.Name]) == 0 {
			l.log.Info("empty phase %s in %s %s",
				phase.Name, l.objectDeployment.GetKind(), l.objectDeployment.GetName())
			continue
		}

		phases[i].Objects = unstructuredSliceToObjectSetObjectSlice(
			l.phaseObjs[phase.Name])
	}
	for phase := range l.phaseObjs {
		if _, ok := knownPhases[phase]; !ok {
			return nil, fmt.Errorf("phase %s not part of ObjectDeployment", phase)
		}
	}
	od.SetPhases(phases)

	return od, nil
}

func (l *packageLoader) walk(fpath string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}

	if strings.HasPrefix(d.Name(), ".") {
		// skip dot-folders and dot-files
		l.log.Info("skipping dot-file or dot-folder", "path", fpath)
		if d.IsDir() {
			// skip dir to prevent loading the directory content
			return filepath.SkipDir
		}
		return nil
	}

	if d.IsDir() {
		// not interested in directories
		return nil
	}

	ext := path.Ext(d.Name())
	if ext != ".yaml" && ext != ".yml" {
		l.log.Info("skipping non .yaml/.yml file", "path", fpath)
		return nil
	}

	objs, err := l.loadKubernetesObjectsFromFile(fpath)
	if err != nil {
		return fmt.Errorf("parsing yaml from %s: %w", fpath, err)
	}

	for i := range objs {
		if err := l.loadObj(objs[i]); err != nil {
			return fmt.Errorf("loading object #%d from %s: %w", i, fpath, err)
		}
	}

	return nil
}

func (l *packageLoader) loadObj(obj unstructured.Unstructured) error {
	if strings.HasSuffix(obj.GetKind(), "ObjectDeployment") {
		if l.objectDeployment != nil {
			return fmt.Errorf("found 2nd ObjectDeployment, should be singleton")
		}

		l.objectDeployment = &obj
		return nil
	}

	if obj.GetAnnotations() == nil ||
		len(obj.GetAnnotations()[phaseAnnotation]) == 0 {
		return fmt.Errorf("missing phase annotation")
	}

	phase := obj.GetAnnotations()[phaseAnnotation]
	l.phaseObjs[phase] = append(l.phaseObjs[phase], obj)

	return nil
}

// Loads kubernetes objects from the given file.
func (l *packageLoader) loadKubernetesObjectsFromFile(filePath string) ([]unstructured.Unstructured, error) {
	fileYaml, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", filePath, err)
	}

	return l.loadKubernetesObjectsFromBytes(fileYaml)
}

// Loads kubernetes objects from given bytes.
// A single file may contain multiple objects separated by "---\n".
func (l *packageLoader) loadKubernetesObjectsFromBytes(fileYaml []byte) (
	[]unstructured.Unstructured, error) {
	// Trim empty starting and ending objects
	fileYaml = bytes.Trim(fileYaml, "---\n")

	var objects []unstructured.Unstructured
	// Split for every included yaml document.
	for i, yamlDocument := range bytes.Split(fileYaml, []byte("---\n")) {
		// templating
		t, err := template.New(fmt.Sprintf("yaml#%d", i)).Parse(string(yamlDocument))
		if err != nil {
			return nil, fmt.Errorf(
				"parsing template from yaml document at index %d: %w", i, err)
		}

		var doc bytes.Buffer
		if err := t.Execute(&doc, l.context); err != nil {
			return nil, fmt.Errorf(
				"executing template from yaml document at index %d: %w", i, err)
		}

		obj := unstructured.Unstructured{}
		if err := yaml.Unmarshal(doc.Bytes(), &obj); err != nil {
			return nil, fmt.Errorf(
				"unmarshalling yaml document at index %d: %w", i, err)
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

func unstructuredSliceToObjectSetObjectSlice(
	objs []unstructured.Unstructured) (out []packagesv1alpha1.ObjectSetObject) {
	for i := range objs {
		out = append(out, packagesv1alpha1.ObjectSetObject{
			Object: runtime.RawExtension{
				Object: &objs[i],
			},
		})
	}
	return
}
