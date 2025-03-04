package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/storageos/kubectl-storageos/pkg/logger"
	pluginutils "github.com/storageos/kubectl-storageos/pkg/utils"
	"github.com/storageos/kubectl-storageos/pkg/version"
)

const airGapInstallWarning = `To install in an air-gap environment, there are some initial steps that are recommended.

- Generate the necessary manifests using the --dry-run flag.
- Update all of the container images to point to the air-gap registry - all of these images will need to be pulled,
  tagged and pushed to the air-gap repository ahead of the actual install. 

To find out more please see the documentation page https://docs.ondat.io/docs/install/airgap/

`

// etcdEndpointsPrompt uses promptui to prompt the user to enter etcd endpoints. The internal validate
// func is run on each character as it is entered as per the regexp - it does not refer to actual
// endpoint validation which is handled later.
func etcdEndpointsPrompt(log *logger.Logger) (string, error) {
	log.Prompt("Please enter ETCD endpoints. If more than one endpoint exists, enter endpoints as a comma-delimited list of machine addresses in the cluster.")
	log.Prompt("Example: 10.42.15.23:2379,10.42.12.22:2379,10.42.13.16:2379")
	log.Prompt("Or please specify ETCD endpoints using the --etcd-endpoints flag.")

	validate := func(input string) error {
		match, _ := regexp.MatchString("^[a-z0-9,.:-]+$", input)
		if !match {
			return errors.New("invalid entry")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "ETCD endpoint(s)",
		Validate: validate,
	}

	return pluginutils.AskUser(prompt, log)
}

// skipNamespaceDeletionPrompt uses promptui to prompt the user to enter decision of skipping namespace deletion
func skipNamespaceDeletionPrompt(log *logger.Logger) (bool, error) {
	log.Warn("Protected namespaces (default, kube-system, kube-node-lease, kube-public) cannot be deleted by kubectl-storageos.")
	log.Prompt("Please confirm namespace deletion or skip using the --skip-namespace-deletion flag.")

	yesValues := map[string]bool{
		"y":   true,
		"yes": true,
	}
	noValues := map[string]bool{
		"":   true,
		"n":  true,
		"no": true,
	}

	validate := func(input string) error {
		ilc := strings.ToLower(input)
		_, yes := yesValues[ilc]
		_, no := noValues[ilc]

		if !yes && !no {
			return errors.New("invalid input")
		}

		return nil
	}
	prompt := promptui.Prompt{
		Label:    "Skip namespace deletion [y/N]",
		Validate: validate,
	}

	input, err := pluginutils.AskUser(prompt, log)
	if err != nil {
		return false, err
	}

	ilc := strings.ToLower(input)
	_, yes := yesValues[ilc]

	return yes, nil
}

// storageClassPrompt uses promptui the user to enter the etcd storage class name
func storageClassPrompt(log *logger.Logger) (string, error) {
	log.Prompt("Please enter the name of the storage class used by the ETCD cluster or specify using the --etcd-storage-class flag.")
	validate := func(input string) error {
		match, _ := regexp.MatchString("^[a-z0-9.-]+$", input)
		if !match {
			return errors.New("invalid entry - must consist only of lowercase alphanumeric characters, '-', or '.'")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "ETCD storage class name",
		Validate: validate,
	}

	return pluginutils.AskUser(prompt, log)
}

// k8sVersionPrompt uses promptui the user to enter the kubernetes version of the target cluster
func k8sVersionPrompt(log *logger.Logger) (string, error) {
	log.Prompt("Please enter the version of the target Kubernetes cluster, enter version string in the format `vX.X.X`.")
	log.Prompt("Example: v1.25.0")
	log.Prompt("Or please specify the version using the --k8s-version flag.")
	validate := func(input string) error {
		match, _ := regexp.MatchString("v[0-9]+.[0-9]+.[0-9]+", input)
		if !match {
			return errors.New("invalid entry, example of valid entry: 'v1.25.0'")
		}
		return nil
	}

	prompt := promptui.Prompt{
		Label:    "Version of target Kubernetes cluster",
		Validate: validate,
	}

	return pluginutils.AskUser(prompt, log)
}

func valueOrDefault(value string, def string) string {
	if value != "" {
		return value
	}
	return def
}

func versionSupportsFeature(existingOperatorVersion, featureFirstSupportedVersion string) error {
	if version.IsDevelop(existingOperatorVersion) {
		return nil
	}

	supported, err := version.IsSupported(existingOperatorVersion, featureFirstSupportedVersion)
	if err != nil {
		return err
	}
	if !supported {
		return fmt.Errorf("feature is not supported in StorageOS %s, requires minimum version %s", existingOperatorVersion, featureFirstSupportedVersion)
	}

	return nil
}

func validateResourceLimit(resourceLimit string) error {
	_, err := resource.ParseQuantity(resourceLimit)
	if err != nil {
		return err
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	return nil
}

func GetBoolIfFlagSet(fs *pflag.FlagSet, flagName string) (*bool, error) {
	if fs.Changed(flagName) {
		enabled, err := fs.GetBool(flagName)
		if err != nil {
			return nil, err
		}
		return &enabled, nil
	}
	return nil, nil
}

func GetBoolIfConfigSet(key string) *bool {
	if viper.IsSet(key) {
		enabled := viper.GetBool(key)
		return &enabled
	}
	return nil
}

func storageosCommand(cmd *cobra.Command, args []string) []string {
	commands := []string{"storageos"}
	if cmd.HasParent() && cmd.Parent().Use != "kubectl-storageos" {
		commands = append(commands, cmd.Parent().Use)
	}
	commands = append(commands, cmd.Use)

	return append(commands, parseArgs(args)...)
}

func parseArgs(args []string) []string {
	parsedArgs := make([]string, 0)
	for _, individualArg := range args {
		if strings.Contains(individualArg, "=") {
			argsSeparated := strings.Split(strings.ReplaceAll(individualArg, "=", "¬"), "¬")
			parsedArgs = append(parsedArgs, argsSeparated...)
			continue
		}
		parsedArgs = append(parsedArgs, individualArg)
	}

	return parsedArgs
}
