#!/bin/bash

: "${TEST_KIND_NODES?= required}"
: "${REPO?= required}"

# This script reads a comma-delimited string TEST_KIND_NODES of storageos/kind-node versions
# for kuttl tests to be run on, and generates the relevant files for each version.

IFS=', ' read -r -a kind_nodes <<< "$TEST_KIND_NODES"


# remove existing files
rm -f ./e2e/kind/*
rm -f ./e2e/kuttl/*
rm -f ./.github/workflows/kuttl*

HEADER="# This file was auto-generated by hack/generate-tests.sh"

for kind_node in "${kind_nodes[@]}"
do
	# write kind config file for version
	major=${kind_node%.*}
	if [ ! -d "./e2e/kind" ]; then
		mkdir -p ./e2e/kind
	fi
	file=./e2e/kind/kind-config-${major}.yaml

	cat <<EOF > "${file}"
${HEADER}
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: storageos/kind-node:v${kind_node}
EOF
	# write kuttl config file for version (upgrade)
	if [ ! -d "./e2e/kuttl" ]; then
		mkdir -p ./e2e/kuttl
	fi
	file=./e2e/kuttl/${REPO}-upgrade-${major}.yaml

    test_dir="stable"
	if [ "$major" == "1.26" ]; then
		test_dir="1.26"
	fi

	cat <<EOF > "${file}"
${HEADER}
apiVersion: kuttl.dev/v1beta1
kind: TestSuite
testDirs:
- ./e2e/tests/upgrade/${test_dir}
kindConfig: e2e/kind/kind-config-${major}.yaml
startKIND: false
timeout: 300
EOF
	# write kuttl config file for version (installer)
	if [ ! -d "./e2e/kuttl" ]; then
		mkdir -p ./e2e/kuttl
	fi
	file=./e2e/kuttl/${REPO}-installer-${major}.yaml

	# installer tests always use 'stable' testDir
	cat <<EOF > "${file}"
${HEADER}
apiVersion: kuttl.dev/v1beta1
kind: TestSuite
testDirs:
- ./e2e/tests/installer/stable
kindConfig: e2e/kind/kind-config-${major}.yaml
startKIND: false
timeout: 300
EOF
	# write kuttl github action for version
	if [ ! -d "./.github/workflows" ]; then
		mkdir -p ./.github/workflows
	fi
	file=./.github/workflows/kuttl-e2e-test-${major}.yaml

	cat <<EOF > "${file}"
${HEADER}
name: kuttl e2e test ${major}

on: [push]

jobs:
  test:
    name: kuttl e2e test ${major}
    runs-on: ubuntu-latest
    env:
      KUTTL: /usr/local/bin/kubectl-kuttl
      KUBECTL_STORAGEOS: /usr/local/bin/kubectl-storageos
    steps:
      - name: Cancel Previous Runs
        uses: styfle/cancel-workflow-action@0.9.1
        with:
          access_token: \${{ github.token }}
      - uses: actions/checkout@v2
      - uses: actions/setup-go@v2
        with:
          go-version: '1.18'
      - name: Install dependencies
        run: |
          sudo curl -Lo \$KUTTL https://github.com/kudobuilder/kuttl/releases/download/v0.13.0/kubectl-kuttl_0.13.0_linux_x86_64
          sudo chmod +x \$KUTTL
          sudo curl -Lo kind https://github.com/kubernetes-sigs/kind/releases/download/v0.15.0/kind-linux-amd64
          sudo chmod +x kind
      - name: Start kind
        run: kind create cluster --retain --wait 2m --config e2e/kind/kind-config-${major}.yaml
      - name: Install kubectl-storageos
        run: |
          make _build
          sudo cp bin/kubectl-storageos \$KUBECTL_STORAGEOS
      - name: Run kuttl installer ${major}
        run: kubectl-kuttl test --config e2e/kuttl/${REPO}-installer-${major}.yaml
      - name: Stop kind
        run: kind delete cluster
      - name: Start kind
        run: kind create cluster --retain --wait 2m --config e2e/kind/kind-config-${major}.yaml
      - name: Run kuttl upgrade ${major}
        run: kubectl-kuttl test --config e2e/kuttl/${REPO}-upgrade-${major}.yaml

      - uses: actions/upload-artifact@v3
        if: \${{ always() }} 
        with:
          name: kind-logs
          path: kind-logs-*
EOF

done
