package installer

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/pkg/errors"
	pluginutils "github.com/storageos/kubectl-storageos/pkg/utils"
)

// EnablePortalManager applies the existing storageoscluster with enablePortalManager set to value of 'enable'.
func (in *Installer) EnablePortalManager(enable bool) error {
	storageOSCluster, err := pluginutils.GetFirstStorageOSCluster(in.clientConfig)
	if err != nil {
		return err
	}
	storageOSClusterManifest, err := storageOSClusterToManifest(storageOSCluster)
	if err != nil {
		return err
	}
	if err := in.fileSys.WriteFile(filepath.Join(stosDir, clusterDir, stosClusterFile), []byte(storageOSClusterManifest)); err != nil {
		return errors.WithStack(err)
	}
	if err := in.enablePortalManager(storageOSCluster.Name, enable); err != nil {
		return err
	}

	return in.kustomizeAndApply(filepath.Join(stosDir, clusterDir), stosClusterFile)
}

func (in *Installer) enablePortalManager(storageOSClusterName string, enable bool) error {
	enablePortalManagerPatch := pluginutils.KustomizePatch{
		Op:    "replace",
		Path:  "/spec/enablePortalManager",
		Value: strconv.FormatBool(enable),
	}

	return in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), stosClusterKind, storageOSClusterName, []pluginutils.KustomizePatch{enablePortalManagerPatch})
}

// InstallPortalManager installs portal manager necessary components.
func (in *Installer) InstallPortalManager() error {
	if err := in.installPortalManagerClient(); err != nil {
		return err
	}
	return in.installPortalManagerConfig()
}

func (in *Installer) installPortalManagerConfig() error {
	if err := in.setFieldInFsManifest(filepath.Join(stosDir, portalConfigDir, kustomizationFile), in.stosConfig.Spec.Install.StorageOSClusterNamespace, "namespace", ""); err != nil {
		return err
	}
	return in.kustomizeAndApply(filepath.Join(stosDir, portalConfigDir), stosPortalConfigFile)
}

func (in *Installer) installPortalManagerClient() error {
	if err := in.setFieldInFsManifest(filepath.Join(stosDir, portalClientDir, kustomizationFile), in.stosConfig.Spec.Install.StorageOSClusterNamespace, "namespace", "secretGenerator", "0"); err != nil {
		return err
	}

	if err := in.setFieldInFsManifest(filepath.Join(stosDir, portalClientDir, kustomizationFile),
		buildStringForKustomize(in.stosConfig.Spec.Install.PortalClientID,
			in.stosConfig.Spec.Install.PortalSecret,
			in.stosConfig.Spec.Install.PortalAPIURL,
			in.stosConfig.Spec.Install.PortalTenantID),
		"literals", "secretGenerator", "0"); err != nil {
		return err
	}
	return in.kustomizeAndApply(filepath.Join(stosDir, portalClientDir), stosPortalClientFile)
}

func buildStringForKustomize(clientID, password, portalURL, tenantID string) string {
	return fmt.Sprint("[",
		"CLIENT_ID", "=", clientID, ",",
		"PASSWORD", "=", password, ",",
		"URL", "=", portalURL, ",",
		"TENANT_ID", "=", tenantID,
		"]")
}

// UninstallPortalManager writes backup-filestem and uninstalls portal manager components.
func (in *Installer) UninstallPortalManager() error {
	storageOSCluster, err := pluginutils.GetFirstStorageOSCluster(in.clientConfig)
	if err != nil {
		return err
	}

	if err := in.writeBackupFileSystem(storageOSCluster); err != nil {
		return err
	}

	if err := in.uninstallPortalManagerConfig(storageOSCluster.Namespace); err != nil {
		return err
	}

	return in.uninstallPortalManagerClient(storageOSCluster.Namespace)
}

func (in *Installer) uninstallPortalManagerClient(storageOSClusterNamespace string) error {
	if err := in.setFieldInFsManifest(filepath.Join(stosDir, portalClientDir, kustomizationFile), storageOSClusterNamespace, "namespace", "secretGenerator", "0"); err != nil {
		return err
	}

	return in.kustomizeAndDelete(filepath.Join(stosDir, portalClientDir), stosPortalClientFile)
}

func (in *Installer) uninstallPortalManagerConfig(storageOSClusterNamespace string) error {
	if err := in.setFieldInFsManifest(filepath.Join(stosDir, portalConfigDir, kustomizationFile), storageOSClusterNamespace, "namespace", ""); err != nil {
		return err
	}

	return in.kustomizeAndDelete(filepath.Join(stosDir, portalConfigDir), stosPortalConfigFile)
}
