package utils

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/blang/semver"
	"github.com/operator-framework/api/pkg/operators"
	olmapiv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/operator-framework/operator-registry/pkg/registry"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
)

type csvName struct {
	Name    string
	Version semver.Version
}
type csvNames []csvName

func (c csvNames) Len() int           { return len(c) }
func (c csvNames) Less(i, j int) bool { return c[i].Version.LT(c[j].Version) }
func (c csvNames) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }

// ReadCSVFromBundleDirectory tries to parse every YAML file in the directory and see if they are CSV.
// According to the strict one CSV rule for every bundle, we return the first file that is considered a CSV type.
func ReadCSVFromBundleDirectory(bundleDir string) (*olmapiv1alpha1.ClusterServiceVersion, string, error) {
	dirContent, err := ioutil.ReadDir(bundleDir)
	if err != nil {
		return nil, "", fmt.Errorf("error reading bundle directory %s, %v", bundleDir, err)
	}

	files := []string{}
	for _, f := range dirContent {
		if !f.IsDir() {
			files = append(files, f.Name())
		}
	}

	for _, file := range files {
		yamlReader, err := os.Open(path.Join(bundleDir, file))
		if err != nil {
			continue
		}

		unstructuredCSV := unstructured.Unstructured{}
		csv := olmapiv1alpha1.ClusterServiceVersion{}

		decoder := k8syaml.NewYAMLOrJSONDecoder(yamlReader, 30)
		if err = decoder.Decode(&unstructuredCSV); err != nil {
			continue
		}

		if unstructuredCSV.GetKind() != operators.ClusterServiceVersionKind {
			continue
		}

		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredCSV.UnstructuredContent(),
			&csv); err != nil {
			return nil, "", err
		}

		return &csv, file, nil
	}
	return nil, "", fmt.Errorf("no ClusterServiceVersion object found in %s", bundleDir)

}

func VerifyManifestDirs(dirs ...string) error {
	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return err
		}

		matches, err := filepath.Glob(dir + "/*.package.yaml")
		if err != nil {
			return err
		}

		if len(matches) == 0 {
			return fmt.Errorf("No package.yaml file found in %s", dir)
		}
	}
	return nil
}

func GetPackageManifest(packageDir string) (*registry.PackageManifest, string, error) {
	matches, err := filepath.Glob(packageDir + "/*.package.yaml")
	if err != nil {
		return nil, "", err
	}

	if len(matches) == 0 {
		return nil, "", fmt.Errorf("No package.yaml file found in %s", packageDir)
	}

	var pkgManifestFile = matches[0]

	pkgManifest := &registry.PackageManifest{}
	err = PopulateObjectFromYAML(pkgManifestFile, &pkgManifest)
	if err != nil {
		return nil, "", err
	}

	return pkgManifest, pkgManifestFile, nil
}

func GetSortedCSVNames(packageDir string) (csvNames, error) {
	bundleDirs, err := ioutil.ReadDir(packageDir)
	var sortedCSVNames csvNames
	if err != nil {
		return nil, err
	}
	for _, bundlePath := range bundleDirs {
		if bundlePath.IsDir() {
			csv, _, err := ReadCSVFromBundleDirectory(filepath.Join(packageDir, bundlePath.Name()))
			if err != nil {
				return nil, err
			}
			sortedCSVNames = append(sortedCSVNames, csvName{Name: csv.Name, Version: csv.Spec.Version.Version})
		}
	}
	sort.Sort(sortedCSVNames)
	return sortedCSVNames, nil
}

func GetCurrentCSV(packageDir string) (*olmapiv1alpha1.ClusterServiceVersion, string, error) {

	pkgManifest, _, err := GetPackageManifest(packageDir)
	if err != nil {
		return nil, "", err
	}

	var currentCSVName string
	for _, channel := range pkgManifest.Channels {
		if channel.IsDefaultChannel(*pkgManifest) {
			currentCSVName = channel.CurrentCSVName
			break
		}
	}

	bundleDirs, err := ioutil.ReadDir(packageDir)
	if err != nil {
		return nil, "", fmt.Errorf("error reading from %s directory, %v", packageDir, err)
	}
	for _, bundlePath := range bundleDirs {
		if bundlePath.IsDir() {
			bundleDir := filepath.Join(packageDir, bundlePath.Name())
			csv, _, err := ReadCSVFromBundleDirectory(bundleDir)
			if err != nil {
				return nil, "", err
			}
			if csv.Name == currentCSVName {
				return csv, bundleDir, nil
			}
		}
	}

	return nil, "", fmt.Errorf("failed to find current csv in %s", packageDir)
}

func UpdatePackageManifest(packageDir, currentCSVName string) (*registry.PackageManifest, error) {

	pkgManifest, pkgManifestFile, err := GetPackageManifest(packageDir)
	if err != nil {
		return nil, err
	}

	pkgManifest.Channels[0].CurrentCSVName = fmt.Sprintf(currentCSVName)
	pkgManifest.DefaultChannelName = pkgManifest.Channels[0].Name

	err = WriteObjectToYAML(pkgManifest, pkgManifestFile)
	if err != nil {
		return nil, err
	}

	return pkgManifest, nil
}
