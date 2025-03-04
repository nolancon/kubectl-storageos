package version

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/blang/semver"
	goversion "github.com/hashicorp/go-version"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/storageos/kubectl-storageos/pkg/consts"
	pluginutils "github.com/storageos/kubectl-storageos/pkg/utils"
)

const (
	oldOperatorYamlUrl = "https://github.com/storageos/cluster-operator/releases/download/%s/storageos-operator.yaml"
	oldClusterYamlUrl  = "https://raw.githubusercontent.com/storageos/cluster-operator/%s/deploy/crds/storageos.com_v1_storageoscluster_cr.yaml"
	oldSecretYamlUrl   = "https://raw.githubusercontent.com/storageos/cluster-operator/%s/deploy/secret.yaml"

	// URLs to install manifests
	stosOperatorManifestsImageUrl = "docker.io/storageos/operator-manifests"
	stosOperatorManifestsUrl      = "https://github.com/storageos/operator/releases/download/%s/storageos-operator.yaml"

	newClusterYamlUrl = "https://github.com/storageos/kubectl-storageos/releases/download/%s/storageos-cluster.yaml"

	resourceQuotaYamlUrl = "https://github.com/storageos/kubectl-storageos/releases/download/%s/resource-quota.yaml"

	portalManagerManifestsImageUrl = "docker.io/storageos/portal-manager-manifests"

	portalSecretYamlUrl = "https://github.com/storageos/kubectl-storageos/releases/download/%s/portal-secret-generator.yaml"

	portalClientYamlUrl = "https://github.com/storageos/kubectl-storageos/releases/download/%s/portal-client-secret-generator.yaml"

	portalConfigYamlUrl = "https://github.com/storageos/kubectl-storageos/releases/download/%s/configmap-storageos-portal-manager.yaml"

	etcdOperatorManifestsImageUrl = "docker.io/storageos/etcd-cluster-operator-manifests"

	etcdOperatorYamlUrl = "https://github.com/storageos/etcd-cluster-operator/releases/download/%s/storageos-etcd-cluster-operator.yaml"

	etcdClusterYamlUrl = "https://github.com/storageos/etcd-cluster-operator/releases/download/%s/storageos-etcd-cluster.yaml"
)

var (
	// EnablePreReleases allows the installer to install pre-release versions of the operator.
	// This setting is defined at build time
	// -X github.com/storageos/kubectl-storageos/pkg/version.EnablePreReleases=true
	EnablePreReleases string
	enablePreReleases bool

	versionRegexp *regexp.Regexp

	PluginVersion string
)

func init() {
	var err error

	if EnablePreReleases != "" {
		enablePreReleases, err = strconv.ParseBool(EnablePreReleases)
		if err != nil {
			panic(err)
		}
	}

	versionRegexp, err = regexp.Compile(consts.VersionRegex)
	if err != nil {
		panic(err)
	}
}

func IsDevelop(version string) bool {
	if version == "develop" || version == "test" {
		return true
	}

	switch len(version) {
	case 56, 64, 96, 128:
		shaRegexp, err := regexp.Compile(consts.ShaVersionRegex)
		if err != nil {
			return false
		}
		return shaRegexp.Match([]byte(version))
	}

	return false
}

func GetExistingOperatorVersion(namespace string) (string, error) {
	oldNS := consts.OldOperatorNamespace
	newNS := consts.NewOperatorNamespace
	if namespace != "" {
		oldNS = namespace
		newNS = namespace
	}
	config, err := pluginutils.NewClientConfig()
	if err != nil {
		return "", err
	}

	clientset, err := pluginutils.GetClientsetFromConfig(config)
	if err != nil {
		return "", errors.Wrap(err, consts.ErrUnableToContructClientFromConfig)
	}

	stosDeployment, errOld := clientset.AppsV1().Deployments(oldNS).Get(context.TODO(), consts.OldOperatorName, metav1.GetOptions{})
	if errOld != nil {
		var errNew error
		stosDeployment, errNew = clientset.AppsV1().Deployments(newNS).Get(context.TODO(), consts.NewOperatorName, metav1.GetOptions{})
		if errNew != nil {
			errNew = errors.Wrap(errNew, errOld.Error())
			return "", errors.Wrap(errNew, "unable to detect existing StorageOS version")
		}
	}
	imageName := stosDeployment.Spec.Template.Spec.Containers[0].Image
	splitImageName := strings.SplitAfter(imageName, ":")
	version := splitImageName[len(splitImageName)-1]

	lessThan, err := VersionIsLessThan(version, consts.OperatorOldestSupportedVersion)
	if err != nil {
		return "", err
	}
	if lessThan {
		return "", fmt.Errorf("kubectl storageos does not support storageos operator version less than %s", consts.OperatorOldestSupportedVersion)
	}

	return version, nil
}

func GetExistingPortalManagerVersion() (string, error) {
	config, err := pluginutils.NewClientConfig()
	if err != nil {
		return "", err
	}

	stosCluster, err := pluginutils.GetFirstStorageOSCluster(config)
	if err != nil {
		return "", err
	}
	clientset, err := pluginutils.GetClientsetFromConfig(config)
	if err != nil {
		return "", errors.Wrap(err, consts.ErrUnableToContructClientFromConfig)
	}
	portalManagerDeployment, err := clientset.AppsV1().Deployments(stosCluster.Namespace).Get(context.TODO(), consts.PortalManagerName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "unable to detect existing Portal Manager version")
	}
	imageName := portalManagerDeployment.Spec.Template.Spec.Containers[0].Image
	splitImageName := strings.SplitAfter(imageName, ":")
	version := splitImageName[len(splitImageName)-1]

	return version, nil
}

func GetExistingEtcdOperatorVersion(namespace string) (string, error) {
	if namespace == "" {
		namespace = consts.EtcdOperatorNamespace
	}
	config, err := pluginutils.NewClientConfig()
	if err != nil {
		return "", err
	}

	clientset, err := pluginutils.GetClientsetFromConfig(config)
	if err != nil {
		return "", errors.Wrap(err, consts.ErrUnableToContructClientFromConfig)
	}
	etcdDeployment, err := clientset.AppsV1().Deployments(namespace).Get(context.TODO(), consts.EtcdOperatorName, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrap(err, "unable to detect existing StorageOS ETCD Operator version")
	}
	imageName := etcdDeployment.Spec.Template.Spec.Containers[0].Image
	splitImageName := strings.SplitAfter(imageName, ":")
	version := splitImageName[len(splitImageName)-1]

	return version, nil
}

func OperatorImageUrlByVersion(operatorVersion string) (string, error) {
	lessThanOrEqual, err := VersionIsLessThanOrEqual(operatorVersion, ClusterOperatorLastVersion())
	if err != nil {
		return "", err
	}
	if lessThanOrEqual {
		return fmt.Sprintf(oldOperatorYamlUrl, operatorVersion), nil
	}

	return fmt.Sprintf("%s:%s", stosOperatorManifestsImageUrl, operatorVersion), nil
}

func ClusterUrlByVersion(operatorVersion string) (string, error) {
	lessThanOrEqual, err := VersionIsLessThanOrEqual(operatorVersion, ClusterOperatorLastVersion())
	if err != nil {
		return "", err
	}
	if lessThanOrEqual {
		return fmt.Sprintf(oldClusterYamlUrl, operatorVersion), nil
	}

	// new storageos-cluster.yaml is located in the operator-manifests image.
	return fmt.Sprintf("%s:%s", stosOperatorManifestsImageUrl, operatorVersion), nil
}

func ResourceQuotaUrlByVersion(operatorVersion string) (string, error) {
	lessThanOrEqual, err := VersionIsLessThanOrEqual(operatorVersion, ClusterOperatorLastVersion())
	if err != nil {
		return "", err
	}
	if lessThanOrEqual {
		return "", nil
	}

	// resource-quota.yaml is located on plugin repo,
	// so we use 'PluginVersion' instead of 'operatorVersion'.
	return fmt.Sprintf(resourceQuotaYamlUrl, PluginVersion), nil
}

func SecretUrlByVersion(operatorVersion string) (string, error) {
	lessThanOrEqual, err := VersionIsLessThanOrEqual(operatorVersion, ClusterOperatorLastVersion())
	if err != nil {
		return "", err
	}
	if lessThanOrEqual {
		return fmt.Sprintf(oldSecretYamlUrl, operatorVersion), nil
	}
	// new operator does not have separate secret and cluster yamls, therefore return empty string

	return "", nil
}

func cleanupVersion(version string) string {
	return versionRegexp.FindString(version)
}

func VersionIsLessThanOrEqual(version, marker string) (bool, error) {
	if IsDevelop(version) {
		return true, nil
	}

	version = cleanupVersion(version)
	marker = cleanupVersion(marker)

	ver, err := goversion.NewVersion(version)
	if err != nil {
		return false, errors.WithStack(err)
	}
	mar, err := goversion.NewVersion(marker)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return ver.LessThanOrEqual(mar), nil
}

func VersionIsLessThan(version, marker string) (bool, error) {
	if IsDevelop(version) {
		return false, nil
	}

	version = cleanupVersion(version)
	marker = cleanupVersion(marker)

	ver, err := goversion.NewVersion(version)
	if err != nil {
		return false, errors.WithStack(err)
	}
	mar, err := goversion.NewVersion(marker)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return ver.LessThan(mar), nil
}

func VersionIsEqualTo(version, marker string) (bool, error) {
	if IsDevelop(version) {
		return false, nil
	}

	version = cleanupVersion(version)
	marker = cleanupVersion(marker)

	ver, err := goversion.NewVersion(version)
	if err != nil {
		return false, errors.WithStack(err)
	}
	mar, err := goversion.NewVersion(marker)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return ver.Equal(mar), nil
}

func OperatorLatestSupportedImageURL() string {
	return fmt.Sprintf("%s:%s", stosOperatorManifestsImageUrl, OperatorLatestSupportedVersion())
}

func OperatorLatestSupportedURL() string {
	return fmt.Sprintf(stosOperatorManifestsUrl, OperatorLatestSupportedVersion())
}

func ClusterLatestSupportedURL() string {
	return fmt.Sprintf(newClusterYamlUrl, PluginVersion)
}

func ResourceQuotaLatestSupportedURL() string {
	return fmt.Sprintf(resourceQuotaYamlUrl, PluginVersion)
}

func PortalManagerLatestSupportedImageURL() string {
	return fmt.Sprintf("%s:%s", portalManagerManifestsImageUrl, PortalManagerLatestSupportedVersion())
}

func PortalSecretLatestSupportedURL() string {
	return fmt.Sprintf(portalSecretYamlUrl, PluginVersion)
}

func PortalClientLatestSupportedURL() string {
	return fmt.Sprintf(portalClientYamlUrl, PluginVersion)
}

func PortalConfigLatestSupportedURL() string {
	return fmt.Sprintf(portalConfigYamlUrl, PluginVersion)
}

func EtcdOperatorLatestSupportedImageURL() string {
	return fmt.Sprintf("%s:%s", etcdOperatorManifestsImageUrl, EtcdOperatorLatestSupportedVersion())
}

func EtcdOperatorLatestSupportedURL() string {
	return fmt.Sprintf(etcdOperatorYamlUrl, EtcdOperatorLatestSupportedVersion())
}

func EtcdClusterLatestSupportedURL() string {
	return fmt.Sprintf(etcdClusterYamlUrl, EtcdOperatorLatestSupportedVersion())
}

// IsSupported takes two versions, current version (haveVersion) and a
// minimum requirement version (wantVersion) and checks if the current version
// is supported by comparing it with the minimum requirement.
func IsSupported(haveVersion, wantVersion string) (bool, error) {
	haveVersion = strings.Trim(versionRegexp.FindString(haveVersion), "v")
	wantVersion = strings.Trim(versionRegexp.FindString(wantVersion), "v")

	supportedVersion, err := semver.Parse(wantVersion)
	if err != nil {
		return false, errors.WithStack(err)
	}

	currentVersion, err := semver.Parse(haveVersion)
	if err != nil {
		return false, errors.WithStack(err)
	}

	return currentVersion.Compare(supportedVersion) >= 0, nil
}
