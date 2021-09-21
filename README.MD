# StorageOS kubectl plugin

[![Go Report Card](https://goreportcard.com/badge/github.com/storageos/kubectl-storageos)](https://goreportcard.com/report/github.com/storageos/kubectl-storageos)
[![e2e test](https://github.com/storageos/kubectl-storageos/actions/workflows/kuttl-e2e-test.yaml/badge.svg)](https://github.com/storageos/kubectl-storageos/actions/workflows/kuttl-e2e-test.yaml)
[![CodeQL](https://github.com/storageos/kubectl-storageos/actions/workflows/codeql-analysis.yml/badge.svg)](https://github.com/storageos/kubectl-storageos/actions/workflows/codeql-analysis.yml)
[![Active](http://img.shields.io/badge/Status-Active-green.svg)](https://github.com/storageos/kubectl-storageos)
[![PR's Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg?style=flat)](https://github.com/storageos/kubectl-storageos/pulls)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

This project is under development, use it at your own risk please.

The main goal of this project is to seemlessly enable StorageOS related administration tasks in a Kubernetes cluster.

## Installation

### Pre-requisites

 * Go 1.16+

Since the project is under active development there isn't a release process or pre-compiled binaries yet.  
For now, to install the plugin you have to build your own version. 

```
# git clone https://github.com/storageos/kubectl-storageos.git
# cd kubectl-storageos
# make build
# cp ./bin/kubectl-storageos /usr/local/bin # Or any directory of your $PATH
```

## Usage

```
# kubectl storageos -h
StorageOS kubectl plugin

Usage:
  storageos [flags]
  storageos [command]

Available Commands:
  bundle      Generate a support bundle
  help        Help about any command
  install     Install StorageOS Cluster Operator
  uninstall   Uninstall StorageOS
  upgrade     Ugrade StorageOS

Flags:
      --as string                      Username to impersonate for the operation
      --as-group stringArray           Group to impersonate for the operation, this flag can be repeated to specify multiple groups.
      --cache-dir string               Default cache directory (default "/root/.kube/cache")
      --certificate-authority string   Path to a cert file for the certificate authority
      --client-certificate string      Path to a client certificate file for TLS
      --client-key string              Path to a client key file for TLS
      --cluster string                 The name of the kubeconfig cluster to use
      --context string                 The name of the kubeconfig context to use
  -h, --help                           help for storageos
      --insecure-skip-tls-verify       If true, the server's certificate will not be checked for validity. This will make your HTTPS connections insecure
      --kubeconfig string              Path to the kubeconfig file to use for CLI requests.
  -n, --namespace string               If present, the namespace scope for this CLI request
      --request-timeout string         The length of time to wait before giving up on a single server request. Non-zero values should contain a corresponding time unit (e.g. 1s, 2m, 3h). A value of zero means don't timeout requests. (default "0")
  -s, --server string                  The address and port of the Kubernetes API server
      --tls-server-name string         Server name to use for server certificate validation. If it is not provided, the hostname used to contact the server is used
      --token string                   Bearer token for authentication to the API server
      --user string                    The name of the kubeconfig user to use

Use "storageos [command] --help" for more information about a command.
```

## Commands

### Install

```
# kubectl storageos install -h

Install StorageOS Cluster Operator

Usage:
  storageos install [flags]

Flags:
      --config-path string               path to look for kubectl-storageos-config.yaml
      --etcd-cluster-yaml string         etcd-cluster.yaml path or url
      --etcd-endpoints string            etcd endpoints
      --etcd-namespace string            namespace of etcd operator and cluster to be installed
      --etcd-operator-yaml string        etcd-operator.yaml path or url
  -h, --help                             help for install
      --include-etcd                     install etcd from github.com/storageos/etcd-cluster-operator (not production-ready)
      --storage-class string             name of storage class to be used by etcd cluster
      --stos-cluster-namespace string    namespace of storageos cluster to be installed
      --stos-cluster-yaml string         storageos-cluster.yaml path or url
      --stos-operator-namespace string   namespace of storageos operator to be installed (default "storageos")
      --stos-operator-yaml string        storageos-operator.yaml path or url
```

Install the latest version of StorageOS on your Kubernetes cluster like so:

`kubectl storageos install`

This command assumes that there is already an ETCD cluster available. As such, the user will be prompted to input the ETCD cluster's endpoints before installation can commence.

Bypass this ETCD endpoint UI prompt like so:

`kubectl storageos install --etcd-endpoints=<etcd-endpoints-here>`


Flags can also be passed to the plugin via the kubectl storageos config file like so:

`kubectl storageos install --config-path=/path/to/config`

For this command to function correctly, the config file should be located and named like so: 

`/path/to/config/kubectl-storageos-config.yaml`

For an example config file, see:

`config/samples/_v1_kubectlstorageosconfig.yaml`



Install an [ETCD cluster](https://github.com/storageos/etcd-cluster-operator) and the latest version of StorageOS like so.

**Warning**: This installation of ETCD is *not* production ready.

`kubectl storageos install --include-etcd`



### Uninstall

```
# kubectl storageos uninstall -h

Uninstall StorageOS

Usage:
  storageos uninstall [flags]

Flags:
      --config-path string               path to look for kubectl-storageos-config.yaml
      --etcd-namespace string            namespace of etcd operator and cluster to be uninstalled
  -h, --help                             help for uninstall
      --include-etcd                     uninstall etcd (only applicable to github.com/storageos/etcd-cluster-operator etcd cluster)
      --stos-cluster-namespace string    namespace of storageos cluster to be uninstalled
      --stos-operator-namespace string   namespace of storageos operator to be uninstalled (default "storageos")
```

Uninstall StorageOS from your Kubernetes cluster like so:

> The following process **will not** remove data stored in disk by StorageOS.
> If Etcd is removed, StorageOS Volumes won't be recoverable, but if the Etcd
> cluster is kept intact, the volumes and their data will be available after a
> reinstall.

`kubectl storageos uninstall`

Uninstall StorageOS and ETCD from your Kubernetes cluster like so:

`kubectl storageos uninstall --include-etcd`

The ETCD uninstallation refers only to an ETCD cluster installed by the StorageOS ETCD Cluster Operator.

**Note**: The StorageOS ETCD Cluster Operator is a fork of the [Improbable Engineering ETCD Cluster Operator](https://github.com/improbable-eng/etcd-cluster-operator). As such, an instance of the latter operator running on the user's Kubernetes cluster can also be uninstalled by this command.

As with the **install** command, flags may be passed to the **uninstall** command using the config file `kubectl-storageos-config.yaml`.


### Upgrade

```
kubectl storageos upgrade -h

Upgrade StorageOS operator and cluster version

Usage:
  storageos upgrade [flags]

Flags:
      --config-path string                         path to look for kubectl-storageos-config.yaml
      --etcd-endpoints string                      etcd endpoints
  -h, --help                                       help for upgrade
      --install-stos-cluster-namespace string      namespace of storageos cluster to be installed
      --install-stos-operator-namespace string     namespace of storageos operator to be installed (default "storageos")
      --stos-cluster-yaml string                   storageos-cluster.yaml path or url to be installed
      --stos-operator-yaml string                  storageos-operator.yaml path or url to be installed
      --uninstall-stos-cluster-namespace string    namespace of storageos cluster to be uninstalled
      --uninstall-stos-operator-namespace string   namespace of storageos operator to be uninstalled
```

Upgrade StorageOS to the latest version like so:

`kubectl storageos upgrade`

The **upgrade** commands uninstalls your existing StorageOS cluster and installs the latest StorageOS cluster.

As with the **install** and **uninstall** commands, all flags can be passed to the **upgrade** command using the config file `kubectl-storageos-config.yaml`.

The **upgrade** command reads the `uninstall` and `install` settings in the config spec to perform the upgrade. 
The following is an example of a config file that might be used for an upgrade with custom namespaces:

```
apiVersion: storageos.com/v1
kind: KubectlStorageOSConfig
metadata:
  name: kubectlstorageosconfig-sample
spec:
  install:
    storageOSOperatorNamespace: storageos-operator-new
    storageOSClusterNamespace: storageos-cluster-new
  uninstall:
    storageOSOperatorNamespace: storageos-operator-test
    storageOSClusterNamespace: storageos-cluster-test
```

## Recovery

Before **uninstall** and **upgrade** commands are executed, a number of manifests relative to the existing StorageOS cluster are written locally to disk in order for the user to manually recover the cluster should an error occur.

These manifests can be located at `$HOME/.kube/storageos`.
