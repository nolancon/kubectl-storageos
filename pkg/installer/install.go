package installer

import (
	"context"
	"encoding/base64"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"sigs.k8s.io/kustomize/api/krusty"

	"github.com/storageos/kubectl-storageos/pkg/consts"
	pluginutils "github.com/storageos/kubectl-storageos/pkg/utils"
)

// Install performs storageos operator and etcd operator installation for kubectl-storageos
func (in *Installer) Install(upgrade bool) error {
	wg := sync.WaitGroup{}
	errChan := make(chan error, 4)

	if in.stosConfig.Spec.IncludeLocalPathProvisioner {
		// This must be done before installing etcd
		errChan <- in.installLocalPathStorageClass()
	}

	if in.stosConfig.Spec.IncludeEtcd {
		wg.Add(1)
		go func() {
			defer wg.Done()

			errChan <- in.installEtcd()
		}()
	} else if !upgrade {
		if err := in.handleEndpointsInput(in.stosConfig.Spec); err != nil {
			return err
		}
	}

	// serialInstall can be set via a build flag, whereas Serial is
	// passed to the plugin via cli flag or config.
	if serialInstall || in.stosConfig.Spec.Serial || in.stosConfig.Spec.Install.DryRun {
		wg.Wait()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		errChan <- in.installStorageOS()
	}()

	wg.Wait()

	if in.stosConfig.Spec.Install.Wait {
		once := sync.Once{}
		errChan <- pluginutils.WaitFor(func() error {
			cluster, err := pluginutils.GetFirstStorageOSCluster(in.clientConfig)
			if err != nil {
				return err
			}

			once.Do(func() {
				in.log.Warnf("Waiting for StorageOS cluster '%s' to enter running phase.", cluster.Name)
			})

			if cluster.Status.Phase != "Running" {
				return fmt.Errorf("cluster %s not ready", cluster.Name)
			}

			return nil
		}, 300, 5)
	}

	go close(errChan)

	return collectErrors(errChan)
}

func (in *Installer) installLocalPathStorageClass() error {
	return in.kustomizeAndApply(filepath.Join(localPathProvisionerDir, storageclassDir), localPathProvisionerFile)
}

func (in *Installer) installEtcd() error {
	var err error
	// add changes to etcd kustomizations here before kustomizeAndApply calls ie make changes
	// to etcd/operator/kustomization.yaml and/or etcd/cluster/kustomization.yaml
	// based on flags (or cli in.stosConfig file)
	if in.stosConfig.Spec.Install.StorageOSOperatorNamespace != consts.NewOperatorNamespace {
		leaderElectionCMNamespacePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/template/spec/containers/0/args/2",
			Value: fmt.Sprintf("%s%s", "--leader-election-cm-namespace=", in.stosConfig.Spec.Install.StorageOSOperatorNamespace),
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, operatorDir, kustomizationFile), "Deployment", consts.EtcdOperatorName, []pluginutils.KustomizePatch{leaderElectionCMNamespacePatch}); err != nil {
			return err
		}
	}
	fsEtcdClusterName, err := in.getFieldInFsMultiDocByKind(filepath.Join(etcdDir, clusterDir, etcdClusterFile), etcdClusterKind, "metadata", "name")
	if err != nil {
		return err
	}

	fsEtcdClusterNamespace, err := in.getFieldInFsManifest(filepath.Join(etcdDir, clusterDir, etcdClusterFile), "metadata", "namespace")
	if err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.EtcdNamespace != fsEtcdClusterNamespace {
		if err = in.setFieldInFsManifest(filepath.Join(etcdDir, operatorDir, kustomizationFile), in.stosConfig.Spec.Install.EtcdNamespace, "namespace", ""); err != nil {
			return err
		}
		if err = in.setFieldInFsManifest(filepath.Join(etcdDir, clusterDir, kustomizationFile), in.stosConfig.Spec.Install.EtcdNamespace, "namespace", ""); err != nil {
			return err
		}
		proxyUrlPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/template/spec/containers/0/args/1",
			Value: fmt.Sprintf("%s%s%s", "--proxy-url=storageos-proxy.", in.stosConfig.Spec.Install.EtcdNamespace, ".svc"),
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, operatorDir, kustomizationFile), "Deployment", consts.EtcdOperatorName, []pluginutils.KustomizePatch{proxyUrlPatch}); err != nil {
			return err
		}

		fsClusterName, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, clusterDir, stosClusterFile), stosClusterKind, "metadata", "name")
		if err != nil {
			return err
		}
		// update endpoint for stos cluster based on etcd namespace flag
		endpointsPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/kvBackend/address",
			Value: fmt.Sprintf("%s%s%s%s", fsEtcdClusterName, ".", in.stosConfig.Spec.Install.EtcdNamespace, ":2379"),
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), stosClusterKind, fsClusterName, []pluginutils.KustomizePatch{endpointsPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdVersionTag != "" {
		versionPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/version",
			Value: in.stosConfig.Spec.Install.EtcdVersionTag,
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{versionPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdTopologyKey != "" {
		preferredTopologyKeyPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/0/podAffinityTerm/topologyKey",
			Value: in.stosConfig.Spec.Install.EtcdTopologyKey,
		}
		preferredWeightPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/affinity/podAntiAffinity/preferredDuringSchedulingIgnoredDuringExecution/0/weight",
			Value: "100",
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{preferredTopologyKeyPatch, preferredWeightPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdReplicas != "" {
		etcdReplicasPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/replicas",
			Value: in.stosConfig.Spec.Install.EtcdReplicas,
		}
		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{etcdReplicasPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdCPULimit != "" {
		cpuLimitsPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/resources/limits/cpu",
			Value: in.stosConfig.Spec.Install.EtcdCPULimit,
		}
		// also change requests to ensure requests = limits and pod is assigned guaranteed qos.
		cpuRequestsPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/resources/requests/cpu",
			Value: in.stosConfig.Spec.Install.EtcdCPULimit,
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{cpuLimitsPatch, cpuRequestsPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdMemoryLimit != "" {
		memoryLimitsPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/resources/limits/memory",
			Value: in.stosConfig.Spec.Install.EtcdMemoryLimit,
		}
		// also change requests to ensure requests = limits and pod is assigned guaranteed qos.
		memoryRequestsPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/podTemplate/resources/requests/memory",
			Value: in.stosConfig.Spec.Install.EtcdMemoryLimit,
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{memoryLimitsPatch, memoryRequestsPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EtcdDockerRepository != "" {
		dockerImagePatch := pluginutils.KustomizePatch{
			Op:    "add",
			Path:  "/spec/template/spec/containers/0/args/-",
			Value: fmt.Sprintf("--etcd-repository=%s", in.stosConfig.Spec.Install.EtcdDockerRepository),
		}
		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, operatorDir, kustomizationFile), "Deployment", consts.EtcdOperatorName, []pluginutils.KustomizePatch{dockerImagePatch}); err != nil {
			return err
		}
	}

	// get the cluster's default storage class if a storage class has not been provided. In any case, add patch
	// with desired storage class name to kustomization for etcd cluster
	if in.stosConfig.Spec.Install.EtcdStorageClassName == "" {
		defaultSc, err := pluginutils.GetDefaultStorageClass(in.clientConfig)
		if err != nil {
			return err
		}
		in.stosConfig.Spec.Install.EtcdStorageClassName = defaultSc.Name
	}

	storageClassPatch := pluginutils.KustomizePatch{
		Op:    "replace",
		Path:  "/spec/storage/volumeClaimTemplate/storageClassName",
		Value: in.stosConfig.Spec.Install.EtcdStorageClassName,
	}
	if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{storageClassPatch}); err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.EtcdTLSEnabled {
		tlsEnabledPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/tls/enabled",
			Value: "true",
		}
		storageOSClusterNSSpecPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/tls/storageOSClusterNamespace",
			Value: in.stosConfig.Spec.Install.StorageOSClusterNamespace,
		}
		storageOSEtcdSecretNamePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/tls/storageOSEtcdSecretName",
			Value: in.stosConfig.Spec.Install.EtcdSecretName,
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(etcdDir, clusterDir, kustomizationFile), etcdClusterKind, fsEtcdClusterName, []pluginutils.KustomizePatch{tlsEnabledPatch, storageOSClusterNSSpecPatch, storageOSEtcdSecretNamePatch}); err != nil {
			return err
		}
	}

	if err = in.kustomizeAndApply(filepath.Join(etcdDir, operatorDir), etcdOperatorFile); err != nil {
		return err
	}
	if err = in.operatorDeploymentsAreReady(filepath.Join(etcdDir, operatorDir, etcdOperatorFile)); err != nil {
		return err
	}

	return in.kustomizeAndApply(filepath.Join(etcdDir, clusterDir), etcdClusterFile)
}

func (in *Installer) installStorageOS() error {
	if err := in.installStorageOSOperator(); err != nil {
		return err
	}
	if err := in.operatorDeploymentsAreReady(filepath.Join(stosDir, operatorDir, stosOperatorFile)); err != nil {
		return err
	}
	if err := in.operatorServicesAreReady(filepath.Join(stosDir, operatorDir, stosOperatorFile)); err != nil {
		return err
	}

	if in.installerOptions.resourceQuota {
		fsResourceQuotaName, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, resourceQuotaDir, resourceQuotaFile), resourceQuotaKind, "metadata", "name")
		if err != nil {
			return err
		}

		clusterNamespacePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/metadata/namespace",
			Value: in.stosConfig.Spec.Install.StorageOSClusterNamespace,
		}

		if err := in.addPatchesToFSKustomize(filepath.Join(stosDir, resourceQuotaDir, kustomizationFile), resourceQuotaKind, fsResourceQuotaName, []pluginutils.KustomizePatch{clusterNamespacePatch}); err != nil {
			return err
		}

		if err = in.kustomizeAndApply(filepath.Join(stosDir, resourceQuotaDir), resourceQuotaFile); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EnablePortalManager {
		if err := in.installPortalManagerClient(in.stosConfig.Spec.Install.StorageOSClusterNamespace); err != nil {
			return err
		}
		if err := in.installPortalManagerConfig(in.stosConfig.Spec.Install.StorageOSClusterNamespace); err != nil {
			return err
		}
		if !in.stosConfig.Spec.SkipStorageOSCluster {
			fsStosClusterName, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, clusterDir, stosClusterFile), stosClusterKind, "metadata", "name")
			if err != nil {
				return err
			}

			if err := in.enablePortalManager(fsStosClusterName, true); err != nil {
				return err
			}
		}
	}

	if in.stosConfig.Spec.SkipStorageOSCluster {
		return nil
	}

	return in.installStorageOSCluster()
}

func (in *Installer) installStorageOSOperator() error {
	var err error
	// add changes to storageos kustomizations here before kustomizeAndApply calls ie make changes
	// to storageos/operator/kustomization.yaml based on flags (or cli in.stosConfig file)
	fsStosOperatorNamespace, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, operatorDir, stosOperatorFile), "Deployment", "metadata", "namespace")
	if err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.StorageOSOperatorNamespace != fsStosOperatorNamespace {
		if err := in.setFieldInFsManifest(filepath.Join(stosDir, operatorDir, kustomizationFile), in.stosConfig.Spec.Install.StorageOSOperatorNamespace, "namespace", ""); err != nil {
			return err
		}
	}

	// if serial install has been set either by build flag or by cli config,
	// set the operator's execution strategy to serial also.
	if in.stosConfig.Spec.Serial || serialInstall {
		if err := in.setExecutionStrategyForOperatorConfigMap(); err != nil {
			return err
		}
	}
	return in.kustomizeAndApply(filepath.Join(stosDir, operatorDir), stosOperatorFile)
}

func (in *Installer) installStorageOSCluster() error {
	var err error
	// add changes to storageos kustomizations here before kustomizeAndApply calls ie make changes
	// to storageos/cluster/kustomization.yaml based on flags (or cli in.stosConfig file)
	if in.stosConfig.Spec.Install.StorageOSClusterNamespace != consts.NewOperatorNamespace {
		if !in.stosConfig.Spec.Install.DryRun {
			if err = in.kubectlClient.Apply(context.TODO(), "", pluginutils.NamespaceYaml(in.stosConfig.Spec.Install.StorageOSClusterNamespace), true); err != nil {
				return err
			}
		}
		if err = in.setFieldInFsManifest(filepath.Join(stosDir, clusterDir, kustomizationFile), in.stosConfig.Spec.Install.StorageOSClusterNamespace, "namespace", ""); err != nil {
			return err
		}
	}

	fsStosClusterName, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, clusterDir, stosClusterFile), stosClusterKind, "metadata", "name")
	if err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.MarkTestCluster {
		testClusterPatch := pluginutils.KustomizePatch{
			Op:    "add", // strategic
			Path:  "/metadata/annotations/ondatTestCluster",
			Value: "true",
		}

		if err := in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), stosClusterKind, fsStosClusterName, []pluginutils.KustomizePatch{testClusterPatch}); err != nil {
			return err
		}
	}

	if err := in.enableMetrics(in.stosConfig.Spec.Install.EnableMetrics); err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.EtcdTLSEnabled {
		tlsEtcdSecretRefNamePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/tlsEtcdSecretRefName",
			Value: in.stosConfig.Spec.Install.EtcdSecretName,
		}
		tlsEtcdSecretRefNamespacePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/spec/tlsEtcdSecretRefNamespace",
			Value: in.stosConfig.Spec.Install.StorageOSClusterNamespace,
		}

		if err = in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), stosClusterKind, fsStosClusterName, []pluginutils.KustomizePatch{tlsEtcdSecretRefNamePatch, tlsEtcdSecretRefNamespacePatch}); err != nil {
			return err
		}
	}

	fsSecretName, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, clusterDir, stosClusterFile), "Secret", "metadata", "name")
	if err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.AdminUsername != "" {
		usernamePatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/data/username",
			Value: base64.StdEncoding.EncodeToString([]byte(in.stosConfig.Spec.Install.AdminUsername)),
		}

		if err := in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), "Secret", fsSecretName, []pluginutils.KustomizePatch{usernamePatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.AdminPassword != "" {
		passwordPatch := pluginutils.KustomizePatch{
			Op:    "replace",
			Path:  "/data/password",
			Value: base64.StdEncoding.EncodeToString([]byte(in.stosConfig.Spec.Install.AdminPassword)),
		}

		if err := in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), "Secret", fsSecretName, []pluginutils.KustomizePatch{passwordPatch}); err != nil {
			return err
		}
	}

	if in.stosConfig.Spec.Install.EnableNodeGuard {
		if err := in.enableNodeGuard(fsStosClusterName); err != nil {
			return err
		}
	}
	return in.kustomizeAndApply(filepath.Join(stosDir, clusterDir), stosClusterFile)
}

// operatorDeploymentsAreReady takes the path of an operator manifest and returns no error if all
// deployments in the manifest have the desired number of ready replicas
func (in *Installer) operatorDeploymentsAreReady(path string) error {
	// return early for dry-run
	if in.stosConfig.Spec.Install.DryRun {
		return nil
	}
	operatorDeployments, err := in.getAllManifestsOfKindFromFsMultiDoc(path, "Deployment")
	if err != nil {
		return err
	}

	for _, deployment := range operatorDeployments {
		deploymentName, err := pluginutils.GetFieldInManifest(deployment, "metadata", "name")
		if err != nil {
			return err
		}
		deploymentNamespace, err := pluginutils.GetFieldInManifest(deployment, "metadata", "namespace")
		if err != nil {
			return err
		}
		if err = pluginutils.WaitFor(func() error {
			return pluginutils.IsDeploymentReady(in.clientConfig, deploymentName, deploymentNamespace)
		}, 120, 5); err != nil {
			return err
		}
	}
	return nil
}

// operatorServicesAreReady takes the path of an operator manifest and returns no error if all
// services in the manifest have a ClusterIP and at least one endpoint that is ready.
func (in *Installer) operatorServicesAreReady(path string) error {
	// return early for dry-run
	if in.stosConfig.Spec.Install.DryRun {
		return nil
	}
	operatorServices, err := in.getAllManifestsOfKindFromFsMultiDoc(path, "Service")
	if err != nil {
		return err
	}

	for _, service := range operatorServices {
		serviceName, err := pluginutils.GetFieldInManifest(service, "metadata", "name")
		if err != nil {
			return err
		}
		serviceNamespace, err := pluginutils.GetFieldInManifest(service, "metadata", "namespace")
		if err != nil {
			return err
		}
		if err = pluginutils.WaitFor(func() error {
			return pluginutils.IsServiceReady(in.clientConfig, serviceName, serviceNamespace)
		}, 90, 5); err != nil {
			return err
		}
	}
	return nil
}

// kustomizeAndApply performs the following in the order described:
// - kustomize run (build) on the provided 'dir'.
// - write the resulting kustomized manifest to dir/file of in-mem fs.
// - remove any namespaces from dir/file of in-mem fs.
// - safely apply the removed namespaces.
// - apply dir/file (once removed namespaces have been applied  successfully).
func (in *Installer) kustomizeAndApply(dir, file string) error {
	kustomizer := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	resMap, err := kustomizer.Run(in.fileSys, dir)
	if err != nil {
		return err
	}
	resYaml, err := resMap.AsYaml()
	if err != nil {
		return err
	}
	if err = in.fileSys.WriteFile(filepath.Join(dir, file), resYaml); err != nil {
		return err
	}

	if in.stosConfig.Spec.Install.DryRun {
		if err := pluginutils.WriteDryRunManifests(fmt.Sprintf("%s%s%s", strconv.Itoa(in.dryRunFileCounter), "-", file), resYaml); err != nil {
			return err
		}
		in.dryRunFileCounter++
		// return early for dry-run without applying manifest
		return nil
	}

	namespaces, err := in.omitAndReturnKindFromFSMultiDoc(filepath.Join(dir, file), "Namespace")
	if err != nil {
		return err
	}
	for _, namespace := range namespaces {
		if err = in.gracefullyApplyNS(namespace); err != nil {
			return err
		}
	}

	manifest, err := in.fileSys.ReadFile(filepath.Join(dir, file))
	if err != nil {
		return err
	}

	return in.kubectlClient.Apply(context.TODO(), "", string(manifest), true)
}

// gracefullyApplyNS applies a namespace and then waits until it has been applied successfully before
// returning no error
func (in *Installer) gracefullyApplyNS(namespaceManifest string) error {
	if err := in.kubectlClient.Apply(context.TODO(), "", namespaceManifest, true); err != nil {
		return err
	}

	namespace, err := pluginutils.GetFieldInManifest(namespaceManifest, "metadata", "name")
	if err != nil {
		return err
	}
	err = pluginutils.WaitFor(func() error {
		_, err := pluginutils.GetNamespace(in.clientConfig, namespace)
		return err
	}, 120, 5)

	return err
}

// enableMetrics sets the boolean for metrics in the in-memory storageos cluster
func (in *Installer) enableMetrics(enable *bool) error {
	if enable == nil {
		// don't override the configfile / defaults
		return nil
	}
	stosClusterYaml, err := in.fileSys.ReadFile(filepath.Join(stosDir, clusterDir, stosClusterFile))
	if err != nil {
		return err
	}

	cluster, secrets, err := pluginutils.OmitAndReturnKindFromMultiDoc(string(stosClusterYaml), "Secret")
	if err != nil {
		return err
	}

	cluster, err = pluginutils.SetFieldInManifest(cluster, strconv.FormatBool(*enable), "enabled", "spec", "metrics")
	if err != nil {
		return err
	}

	return in.fileSys.WriteFile(filepath.Join(stosDir, clusterDir, stosClusterFile), []byte(makeMultiDoc(append(secrets, cluster)...)))
}

// setExecutionStrategyForOperatorConfigMap sets the OperatorConfig's serialExecutionStrategy
// field to true. This ensures that the operator will install components serially.
// Setting this value is greatly complicated by the fact that the OperatorConfig is stored
// in the storageos-operator configmap - see comments throughout the function to help
// describe the process.
func (in *Installer) setExecutionStrategyForOperatorConfigMap() error {
	stosOperatorYaml, err := in.fileSys.ReadFile(filepath.Join(stosDir, operatorDir, stosOperatorFile))
	if err != nil {
		return err
	}

	// We need to get the storageos-operator configmap.
	// There is more than one configmap in storageos-operator.yaml, so first
	// we will get all configmaps by kind. Then we will get the configmap we want
	// by name (storageos-operator) - We cannot search by name initially as numerous
	// objects of different kinds have that name, but only one configmap.
	stosOperatorYamlConfigMaps, err := pluginutils.GetAllManifestsOfKindFromMultiDoc(string(stosOperatorYaml), "ConfigMap")
	if err != nil {
		return err
	}
	stosOperatorConfigMap, err := pluginutils.GetManifestFromMultiDocByName(makeMultiDoc(stosOperatorYamlConfigMaps...), "storageos-operator")
	if err != nil {
		return err
	}

	// Now that we have the configmap we want, we can extract the manifest from it's data
	// that we need to transform. This is the OperatorConfig object.
	operatorConfig, err := pluginutils.GetManifestFromConfigMapData(stosOperatorConfigMap)
	if err != nil {
		return err
	}

	// Set the serialExecutionStrategy in the OperatorConfig.
	// Then set the updated OperatorConfig back in the configmap's data.
	operatorConfig, err = pluginutils.SetFieldInManifest(operatorConfig, "true", "serialExecutionStrategy")
	if err != nil {
		return err
	}
	stosOperatorConfigMap, err = pluginutils.SetManifestInConfigMapData(stosOperatorConfigMap, "operator_config.yaml", operatorConfig)
	if err != nil {
		return err
	}

	// Now we need to replace the existing configmap in storageos-operator.yaml
	// with the updated one which we have edited.
	// To do this, we must first take the slice of all configmaps in storageos-operator.yaml
	// and remove the 'old' storageos-operator configmap.
	// Then swap in the 'new' storageos-operator configmap to our slice of configmaps.
	// Next we get the existing storageos-operator.yaml WITHOUT any configmaps, and
	// swap in the slice of configmaps we have created (containing the new storageos-operator configmap).
	// Finally, we write the updated storageos-operator.yaml to the in-memory fs in its entirity
	stosOperatorYamlConfigMapsWithoutOperatorConfig, err := pluginutils.OmitByNameFromMultiDoc(makeMultiDoc(stosOperatorYamlConfigMaps...), "storageos-operator")
	if err != nil {
		return err
	}
	stosOperatorYamlConfigMaps = []string{stosOperatorYamlConfigMapsWithoutOperatorConfig, stosOperatorConfigMap}
	stosOperatorYamlWithoutConfigMaps, err := pluginutils.OmitKindFromMultiDoc(string(stosOperatorYaml), "ConfigMap")
	if err != nil {
		return err
	}

	return in.fileSys.WriteFile(filepath.Join(stosDir, operatorDir, stosOperatorFile),
		[]byte(makeMultiDoc(append(stosOperatorYamlConfigMaps, stosOperatorYamlWithoutConfigMaps)...)))
}

// enableNodeGuard sets nodeGuard in nodeManagerFeatures in cluster spec.
// This can be simply enabling node guard: 'nodeGuard: ""'
// Or enabling node guard with env vars:   'nodeGuard: "FOO=foo,BAR=bar"'
func (in *Installer) enableNodeGuard(storageOSClusterName string) error {
	nodeManagerFeatures, err := in.getFieldInFsMultiDocByKind(filepath.Join(stosDir, clusterDir, stosClusterFile), stosClusterKind, "spec", "nodeManagerFeatures")
	if err != nil {
		return err
	}

	if strings.Contains(nodeManagerFeatures, "nodeGuard:") && in.stosConfig.Spec.Install.NodeGuardEnv == "" {
		// nodeGuard is already enabled and we have not specified any new env vars.
		// Nothing to do so return to prevent overwriting any existing env vars.
		return nil
	}
	// nodeManagerFeatures, if not empty, will be a string that looks like:
	// "foo: foo
	// bar: bar"
	//
	// We need to turn this into:
	// "foo: foo,bar: bar,"
	nodeManagerFeatures = strings.ReplaceAll(nodeManagerFeatures, "\n", ",")
	if nodeManagerFeatures != "" {
		nodeManagerFeatures += ","
	}

	// Then append the new nodeGuard values, ending up with a map:
	// "{foo: foo,bar: bar,nodeGuard: "FOO=foo,BAR=bar"}".
	// Put existing features at the beginning of the map so that if nodeGuard already exists we
	// will overwrite it with the new values (this 'Value' behaves like a normal map when kustomize is run).
	nodeManagerFeatures = "{" + nodeManagerFeatures + "nodeGuard: \"" + in.stosConfig.Spec.Install.NodeGuardEnv + "\"}"

	nodeGuardPatch := pluginutils.KustomizePatch{
		Op:    "add",
		Path:  "/spec/nodeManagerFeatures",
		Value: nodeManagerFeatures,
	}

	return in.addPatchesToFSKustomize(filepath.Join(stosDir, clusterDir, kustomizationFile), stosClusterKind, storageOSClusterName, []pluginutils.KustomizePatch{nodeGuardPatch})
}
