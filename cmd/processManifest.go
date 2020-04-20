package cmd

import (
	"fmt"

	"github.com/integr8ly/delorean/pkg/utils"
	olmapiv1alpha1 "github.com/operator-framework/operator-lifecycle-manager/pkg/api/apis/operators/v1alpha1"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
)

var manifestDir string

const (
	envVarWatchNamespace = "WATCH_NAMESPACE"
	envVarNamespace      = "NAMESPACE"
)

// processManifestCmd represents the processImageManifests command
var processManifestCmd = &cobra.Command{
	Use:   "process-manifest",
	Short: "Process a given manifest to meet the rhmi requirements.",
	Long:  `Process a given manifest to meet the rhmi requirements.`,
	Run: func(cmd *cobra.Command, args []string) {
		//verify it's a manifest dir.
		err := utils.VerifyManifestDirs(manifestDir)
		if err != nil {
			handleError(err)
		}
		//get the latest csv
		csv, bundleDir, err := utils.GetCurrentCSV(manifestDir)
		if err != nil {
			handleError(err)
		}

		//get csvfile
		csv, filename, err := utils.ReadCSVFromBundleDirectory(bundleDir)
		if err != nil {
			handleError(err)
		}
		if filename == "" {
			handleError(fmt.Errorf("No csv file found in the directory"))
		}

		//populate a csv object from the file
		filepath := fmt.Sprintf("%s/%s", bundleDir, filename)

		err = utils.PopulateObjectFromYAML(filepath, csv)
		if err != nil {
			handleError(err)
		}
		// make the updates to that file
		// TODO Get the correct replaces value and update it.
		csv.Spec.Replaces = "Some Replaces"
		updateEnvs(csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs)
		//write the file out using the object
		err = utils.WriteObjectToYAML(csv, filepath)
		if err != nil {
			handleError(err)
		}
	},
}

func init() {
	ewsCmd.AddCommand(processManifestCmd)

	processManifestCmd.Flags().StringVarP(&manifestDir, "manifest-dir", "m", "", "Manifest Directory Location.")
}

func updateEnvs(deployments []olmapiv1alpha1.StrategyDeploymentSpec) {
	spec := deployments[0].Spec
	envs := spec.Template.Spec.Containers[0].Env
	watchNamespaceEnv := v1.EnvVar{
		Name: envVarWatchNamespace,
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['olm.targetNamespaces']",
			},
		},
	}
	namespaceEnv := v1.EnvVar{
		Name: envVarNamespace,
		ValueFrom: &v1.EnvVarSource{
			FieldRef: &v1.ObjectFieldSelector{
				FieldPath: "metadata.annotations['olm.targetNamespaces']",
			},
		},
	}
	for i, env := range envs {
		if env.Name == envVarWatchNamespace {
			envs[i] = watchNamespaceEnv
		}
		if env.Name == envVarNamespace {
			envs[i] = namespaceEnv
		}
	}
	fmt.Print("Finished updating envs")
}
