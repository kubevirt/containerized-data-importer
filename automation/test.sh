#!/bin/bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2017 Red Hat, Inc.
#

# CI considerations: $TARGET is used by the jenkins build, to distinguish what to test
# Currently considered $TARGET values:
#     kubernetes-release: Runs all functional tests on a release kubernetes setup
#     openshift-release: Runs all functional tests on a release openshift setup

set -ex

export NAMESPACE="cdi-$(cat /dev/urandom | tr -dc 'a-z0-9' | fold -w 5 | head -n 1)"
export CDI_NAMESPACE=$NAMESPACE

echo "namespace: ${NAMESPACE}, cdi-namespace: ${CDI_NAMESPACE}"

readonly ARTIFACTS_PATH="${ARTIFACTS}"
readonly BAZEL_CACHE="${BAZEL_CACHE:-http://bazel-cache.kubevirt-prow.svc.cluster.local:8080/kubevirt.io/containerized-data-importer}"

export KUBEVIRT_PROVIDER=$TARGET

if [[ $TARGET =~ openshift-.* ]]; then
  export KUBEVIRT_PROVIDER="os-3.11.0-crio"
elif [[ $TARGET =~ k8s-.* ]]; then
  export KUBEVIRT_NUM_NODES=2
  export KUBEVIRT_MEMORY_SIZE=8192
fi

if [ ! -d "cluster-up/cluster/$KUBEVIRT_PROVIDER" ]; then
  echo "The cluster provider $KUBEVIRT_PROVIDER does not exist"
  exit 1
fi

export UPGRADE_FROM=$(curl -s https://github.com/kubevirt/containerized-data-importer/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
echo "Upgrading from verions: $UPGRADE_FROM"
export KUBEVIRT_NUM_NODES=2

kubectl() { cluster-up/kubectl.sh "$@"; }

export CDI_NAMESPACE="${CDI_NAMESPACE:-cdi}"

make cluster-down
# Create .bazelrc to use remote cache
cat >.bazelrc <<EOF
startup --host_jvm_args=-Dbazel.DigestFunction=sha256
build --remote_local_fallback
build --remote_http_cache=${BAZEL_CACHE}
build --jobs=4
EOF

make cluster-up

# Wait for nodes to become ready
set +e
kubectl_rc=0
retry_counter=0
while [[ $retry_counter -lt 30 ]] && [[ $kubectl_rc -ne 0 || -n "$(kubectl get nodes --no-headers | grep NotReady)" ]]; do
    echo "Waiting for all nodes to become ready ..."
    kubectl get nodes --no-headers
    kubectl_rc=$?
    retry_counter=$((retry_counter + 1))
    sleep 10
done
set -e

if [ $retry_counter -eq 30 ]; then
	echo "Not all nodes are up"
	exit 1
fi

echo "Nodes are ready:"
kubectl get nodes

make cluster-sync

kubectl version

ginko_params="--test-args=--ginkgo.noColor --junit-output=${ARTIFACTS_PATH}/junit.functest.xml"

# Run functional tests
TEST_ARGS=$ginko_params make test-functional-ci
