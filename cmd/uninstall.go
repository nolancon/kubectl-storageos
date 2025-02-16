package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	apiv1 "github.com/storageos/kubectl-storageos/api/v1"
	"github.com/storageos/kubectl-storageos/pkg/consts"
	"github.com/storageos/kubectl-storageos/pkg/installer"
	"github.com/storageos/kubectl-storageos/pkg/logger"
	pluginutils "github.com/storageos/kubectl-storageos/pkg/utils"
	pluginversion "github.com/storageos/kubectl-storageos/pkg/version"
)

const uninstall = "uninstall"

func UninstallCmd() *cobra.Command {
	var err error
	var traceError bool
	pluginLogger := logger.NewLogger()
	cmd := &cobra.Command{
		Use:          uninstall,
		Args:         cobra.MinimumNArgs(0),
		Short:        "Uninstall StorageOS and (optionally) ETCD",
		Long:         `Uninstall StorageOS and (optionally) ETCD`,
		SilenceUsage: true,
		PreRun:       func(cmd *cobra.Command, args []string) {},
		Run: func(cmd *cobra.Command, args []string) {
			defer pluginutils.ConvertPanicToError(func(e error) {
				err = e
			})
			config := &apiv1.KubectlStorageOSConfig{}
			err = setUninstallValues(cmd, config)
			if err != nil {
				return
			}

			traceError = config.Spec.StackTrace

			err = uninstallCmd(config, pluginutils.HasFlagSet(installer.SkipNamespaceDeletionFlag), pluginLogger)
		},
		PostRunE: func(cmd *cobra.Command, args []string) error {
			if err = pluginutils.HandleError(uninstall, err, traceError); err != nil {
				pluginLogger.Error(fmt.Sprintf("%s%s", uninstall, " has failed"))
				return err
			}
			pluginLogger.Success("StorageOS uninstalled successfully.")
			return nil
		},
	}
	cmd.Flags().Bool(installer.StackTraceFlag, false, "print stack trace of error")
	cmd.Flags().BoolP(installer.VerboseFlag, "v", false, "verbose logging")
	cmd.Flags().Bool(installer.SkipNamespaceDeletionFlag, false, "leave namespaces untouched")
	cmd.Flags().Bool(installer.SkipExistingWorkloadCheckFlag, false, "skip check for PVCs using storageos storage class during uninstall")
	cmd.Flags().Bool(installer.SkipStosClusterFlag, false, "skip storageos cluster uninstallation")
	cmd.Flags().Bool(installer.IncludeEtcdFlag, false, "uninstall etcd (only applicable to github.com/storageos/etcd-cluster-operator etcd cluster)")
	cmd.Flags().String(installer.EtcdNamespaceFlag, consts.EtcdOperatorNamespace, "namespace of etcd operator and cluster to be uninstalled")
	cmd.Flags().String(installer.StosOperatorNSFlag, consts.NewOperatorNamespace, "namespace of storageos operator to be uninstalled")
	cmd.Flags().String(installer.StosConfigPathFlag, "", "path to look for kubectl-storageos-config.yaml")
	cmd.Flags().String(installer.StosOperatorYamlFlag, "", "storageos-operator.yaml path or url")
	cmd.Flags().String(installer.StosClusterYamlFlag, "", "storageos-cluster.yaml path or url")
	cmd.Flags().String(installer.StosPortalConfigYamlFlag, "", "storageos-portal-manager-configmap.yaml path or url")
	cmd.Flags().String(installer.StosPortalClientSecretYamlFlag, "", "storageos-portal-manager-client-secret.yaml path or url")
	cmd.Flags().String(installer.EtcdClusterYamlFlag, "", "etcd-cluster.yaml path or url")
	cmd.Flags().String(installer.EtcdOperatorYamlFlag, "", "etcd-operator.yaml path or url")
	cmd.Flags().String(installer.ResourceQuotaYamlFlag, "", "resource-quota.yaml path or url")
	cmd.Flags().Bool(installer.IncludeLocalPathProvisionerFlag, false, "uninstall local path provisioner storage class")
	cmd.Flags().String(installer.LocalPathProvisionerYamlFlag, "", "local-path-provisioner.yaml path or url")
	cmd.Flags().Bool(installer.SerialFlag, false, "uninstall components serially")
	cmd.Flags().Bool(installer.AirGapFlag, false, "uninstall in an air gapped environment")
	cmd.Flags().String(installer.StosVersionFlag, "", "version of storageos operator to uninstall")
	cmd.Flags().String(installer.EtcdOperatorVersionFlag, "", "version of etcd operator to uninstall")
	cmd.Flags().String(installer.PortalManagerVersionFlag, "", "version of portal manager to uninstall")

	viper.BindPFlags(cmd.Flags())

	return cmd
}

func uninstallCmd(config *apiv1.KubectlStorageOSConfig, skipNamespaceDeletionHasSet bool, log *logger.Logger) error {
	log.Verbose = config.Spec.Verbose

	var err error
	// if skip namespace delete was not passed via flag or config, prompt user to enter manually
	if !config.Spec.SkipNamespaceDeletion && !skipNamespaceDeletionHasSet {
		var err error
		config.Spec.SkipNamespaceDeletion, err = skipNamespaceDeletionPrompt(log)
		if err != nil {
			return err
		}
	}

	if config.Spec.Uninstall.StorageOSVersion == "" {
		config.Spec.Uninstall.StorageOSVersion, err = pluginversion.GetExistingOperatorVersion(config.Spec.Uninstall.StorageOSOperatorNamespace)
		if err != nil {
			return err
		}
		log.Successf("Discovered StorageOS cluster and operator version '%s'.", config.Spec.Uninstall.StorageOSVersion)
	}

	if config.Spec.Uninstall.PortalManagerVersion == "" {
		config.Spec.Uninstall.PortalManagerVersion, err = pluginversion.GetExistingPortalManagerVersion()
		if err != nil {
			// unable to get portal manager version. It may not exist, so just log a warning and continue.
			log.Warnf("%s. Continuing uninstall.", err.Error())
		} else {
			log.Successf("Discovered Portal Manager version '%s'.", config.Spec.Uninstall.PortalManagerVersion)
		}
	}

	if config.Spec.IncludeEtcd {
		if config.Spec.Uninstall.EtcdOperatorVersion == "" {
			config.Spec.Uninstall.EtcdOperatorVersion, err = pluginversion.GetExistingEtcdOperatorVersion(config.Spec.Uninstall.EtcdNamespace)
			if err != nil {
				return err
			}
			log.Successf("Discovered ETCD cluster and operator version '%s'.", config.Spec.Uninstall.EtcdOperatorVersion)
		}
	}

	if err := installer.FlagsAreSet(uninstallFlagsFilter(config)); err != nil {
		return err
	}

	if err = setVersionSpecificValues(config); err != nil {
		return err
	}

	cliInstaller, err := installer.NewUninstaller(config, log)
	if err != nil {
		return err
	}

	log.Commencing(uninstall)
	return cliInstaller.Uninstall(false)
}

func setUninstallValues(cmd *cobra.Command, config *apiv1.KubectlStorageOSConfig) error {
	viper.BindPFlag(installer.StosConfigPathFlag, cmd.Flags().Lookup(installer.StosConfigPathFlag))
	v := viper.GetViper()
	viper.SetConfigName("kubectl-storageos-config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(v.GetString(installer.StosConfigPathFlag))

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file was found but another error was produced
			return fmt.Errorf("error discovered in config file: %v", err)
		}
		// Config file not found; set fields in new config object directly
		config.Spec.StackTrace, err = cmd.Flags().GetBool(installer.StackTraceFlag)
		if err != nil {
			return err
		}
		config.Spec.Verbose, err = cmd.Flags().GetBool(installer.VerboseFlag)
		if err != nil {
			return err
		}
		config.Spec.SkipNamespaceDeletion, err = cmd.Flags().GetBool(installer.SkipNamespaceDeletionFlag)
		if err != nil {
			return err
		}
		config.Spec.SkipExistingWorkloadCheck, err = cmd.Flags().GetBool(installer.SkipExistingWorkloadCheckFlag)
		if err != nil {
			return err
		}
		config.Spec.IncludeEtcd, err = cmd.Flags().GetBool(installer.IncludeEtcdFlag)
		if err != nil {
			return err
		}
		config.Spec.SkipStorageOSCluster, err = cmd.Flags().GetBool(installer.SkipStosClusterFlag)
		if err != nil {
			return err
		}
		config.Spec.IncludeLocalPathProvisioner, err = cmd.Flags().GetBool(installer.IncludeLocalPathProvisionerFlag)
		if err != nil {
			return err
		}
		config.Spec.Serial, err = cmd.Flags().GetBool(installer.SerialFlag)
		if err != nil {
			return err
		}
		config.Spec.AirGap, err = cmd.Flags().GetBool(installer.AirGapFlag)
		if err != nil {
			return err
		}

		config.Spec.Uninstall.StorageOSOperatorNamespace = cmd.Flags().Lookup(installer.StosOperatorNSFlag).Value.String()
		config.Spec.Uninstall.EtcdNamespace = cmd.Flags().Lookup(installer.EtcdNamespaceFlag).Value.String()
		config.Spec.Uninstall.StorageOSOperatorYaml = cmd.Flags().Lookup(installer.StosOperatorYamlFlag).Value.String()
		config.Spec.Uninstall.StorageOSClusterYaml = cmd.Flags().Lookup(installer.StosClusterYamlFlag).Value.String()
		config.Spec.Uninstall.StorageOSPortalConfigYaml = cmd.Flags().Lookup(installer.StosPortalConfigYamlFlag).Value.String()
		config.Spec.Uninstall.StorageOSPortalClientSecretYaml = cmd.Flags().Lookup(installer.StosPortalClientSecretYamlFlag).Value.String()
		config.Spec.Uninstall.EtcdOperatorYaml = cmd.Flags().Lookup(installer.EtcdOperatorYamlFlag).Value.String()
		config.Spec.Uninstall.EtcdClusterYaml = cmd.Flags().Lookup(installer.EtcdClusterYamlFlag).Value.String()
		config.Spec.Uninstall.ResourceQuotaYaml = cmd.Flags().Lookup(installer.ResourceQuotaYamlFlag).Value.String()
		config.Spec.Uninstall.LocalPathProvisionerYaml = cmd.Flags().Lookup(installer.LocalPathProvisionerYamlFlag).Value.String()
		config.Spec.Uninstall.StorageOSVersion = cmd.Flags().Lookup(installer.StosVersionFlag).Value.String()
		config.Spec.Uninstall.EtcdOperatorVersion = cmd.Flags().Lookup(installer.EtcdOperatorVersionFlag).Value.String()
		config.Spec.Uninstall.PortalManagerVersion = cmd.Flags().Lookup(installer.PortalManagerVersionFlag).Value.String()

		return nil
	}
	// config file read without error, set fields in new config object
	config.Spec.StackTrace = viper.GetBool(installer.StackTraceConfig)
	config.Spec.Verbose = viper.GetBool(installer.VerboseConfig)
	config.Spec.SkipNamespaceDeletion = viper.GetBool(installer.SkipNamespaceDeletionConfig)
	config.Spec.SkipExistingWorkloadCheck = viper.GetBool(installer.SkipExistingWorkloadCheckConfig)
	config.Spec.SkipStorageOSCluster = viper.GetBool(installer.SkipStosClusterConfig)
	config.Spec.IncludeEtcd = viper.GetBool(installer.IncludeEtcdConfig)
	config.Spec.Serial = viper.GetBool(installer.SerialConfig)
	config.Spec.AirGap = viper.GetBool(installer.AirGapConfig)
	config.Spec.Uninstall.StorageOSOperatorNamespace = viper.GetString(installer.UninstallStosOperatorNSConfig)
	config.Spec.Uninstall.EtcdNamespace = valueOrDefault(viper.GetString(installer.UninstallEtcdNSConfig), consts.EtcdOperatorNamespace)
	config.Spec.Uninstall.StorageOSOperatorYaml = viper.GetString(installer.UninstallStosOperatorYamlConfig)
	config.Spec.Uninstall.StorageOSClusterYaml = viper.GetString(installer.UninstallStosClusterYamlConfig)
	config.Spec.Uninstall.StorageOSPortalConfigYaml = viper.GetString(installer.UninstallStosPortalConfigYamlConfig)
	config.Spec.Uninstall.StorageOSPortalClientSecretYaml = viper.GetString(installer.UninstallStosPortalClientSecretYamlConfig)
	config.Spec.Uninstall.EtcdOperatorYaml = viper.GetString(installer.UninstallEtcdOperatorYamlConfig)
	config.Spec.Uninstall.EtcdClusterYaml = viper.GetString(installer.UninstallEtcdClusterYamlConfig)
	config.Spec.Uninstall.ResourceQuotaYaml = viper.GetString(installer.UninstallResourceQuotaYamlConfig)
	config.Spec.IncludeLocalPathProvisioner = viper.GetBool(installer.IncludeLocalPathProvisionerConfig)
	config.Spec.Uninstall.LocalPathProvisionerYaml = viper.GetString(installer.UninstallLocalPathProvisionerYamlConfig)
	config.Spec.Uninstall.StorageOSVersion = viper.GetString(installer.UninstallStosVersionConfig)
	config.Spec.Uninstall.EtcdOperatorVersion = viper.GetString(installer.UninstallEtcdOperatorVersionConfig)
	config.Spec.Uninstall.PortalManagerVersion = viper.GetString(installer.UninstallPortalManagerVersionConfig)

	return nil
}

func setVersionSpecificValues(config *apiv1.KubectlStorageOSConfig) (err error) {
	// Don't fetch version specific manifests for develop edition
	if pluginversion.IsDevelop(config.Spec.Uninstall.StorageOSVersion) {
		return
	}

	// set additional values to be used by Installer for in memory fs build
	if config.Spec.Uninstall.StorageOSOperatorYaml == "" {
		config.Spec.Uninstall.StorageOSOperatorYaml, err = pluginversion.OperatorImageUrlByVersion(config.Spec.Uninstall.StorageOSVersion)
		if err != nil {
			return
		}
	}

	if config.Spec.Uninstall.StorageOSClusterYaml == "" {
		config.Spec.Uninstall.StorageOSClusterYaml, err = pluginversion.ClusterUrlByVersion(config.Spec.Uninstall.StorageOSVersion)
		if err != nil {
			return
		}
	}
	if config.Spec.Uninstall.ResourceQuotaYaml == "" {
		config.Spec.Uninstall.ResourceQuotaYaml, err = pluginversion.ResourceQuotaUrlByVersion(config.Spec.Uninstall.StorageOSVersion)
		if err != nil {
			return
		}
	}

	config.InstallerMeta.StorageOSSecretYaml, err = pluginversion.SecretUrlByVersion(config.Spec.Uninstall.StorageOSVersion)
	if err != nil {
		return
	}

	return
}

func uninstallFlagsFilter(config *apiv1.KubectlStorageOSConfig) map[string]string {
	requiredFlags := make(map[string]string)
	if !config.Spec.AirGap {
		return requiredFlags
	}

	if config.Spec.Install.StorageOSVersion == "" {
		// no stos version was found in the cluster, so it becomes a requirement
		// for an air-gap uninstall to avoid querying for the latest version.
		requiredFlags[installer.StosVersionFlag] = config.Spec.Install.StorageOSVersion
	}

	if config.Spec.IncludeEtcd && config.Spec.Install.EtcdOperatorVersion == "" {
		// no etcd-operator version was found in the cluster, so it becomes a requirement
		// for an air-gap uninstall to avoid querying for the latest version.
		requiredFlags[installer.EtcdOperatorVersionFlag] = config.Spec.Install.EtcdOperatorVersion
	}

	if config.Spec.IncludeLocalPathProvisioner {
		// local path provisioner spec is pulled from a URL, so it becomes a requirement for
		// an air-gap uninstall to avoid download attempt.
		requiredFlags[installer.LocalPathProvisionerYamlFlag] = config.Spec.Install.LocalPathProvisionerYaml
	}

	return requiredFlags
}
