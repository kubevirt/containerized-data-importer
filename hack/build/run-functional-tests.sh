#!/usr/bin/env bash

#Copyright 2018 The CDI Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

set -eo pipefail

readonly MAX_CDI_WAIT_RETRY=30
readonly CDI_WAIT_TIME=10

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source hack/build/config.sh
source hack/build/common.sh
source cluster-up/hack/common.sh

KUBEVIRTCI_CONFIG_PATH="$(
    cd "$(dirname "$BASH_SOURCE[0]")/../../"
    echo "$(pwd)/_ci-configs"
)"

# functional testing
BASE_PATH=${KUBEVIRTCI_CONFIG_PATH:-$PWD}
KUBECONFIG=${KUBECONFIG:-$BASE_PATH/$KUBEVIRT_PROVIDER/.kubeconfig}
GOCLI=${GOCLI:-${CDI_DIR}/cluster-up/cli.sh}
KUBE_MASTER_URL=${KUBE_MASTER_URL:-""}
CDI_NAMESPACE=${CDI_NAMESPACE:-cdi}
SNAPSHOT_SC=${SNAPSHOT_SC:-rook-ceph-block}
BLOCK_SC=${BLOCK_SC:-rook-ceph-block}

if [ -z "${KUBECTL+x}" ]; then
    kubevirtci_kubectl="${BASE_PATH}/${KUBEVIRT_PROVIDER}/.kubectl"
    if [ -e ${kubevirtci_kubectl} ]; then
        KUBECTL=${kubevirtci_kubectl}
    else
        KUBECTL=$(which kubectl)
    fi
fi

# parsetTestOpts sets 'pkgs' and test_args
parseTestOpts "${@}"

echo $KUBECONFIG
echo $KUBECTL

arg_master="${KUBE_MASTER_URL:+-master=$KUBE_MASTER_URL}"
arg_namespace="${CDI_NAMESPACE:+-cdi-namespace=$CDI_NAMESPACE}"
arg_kubeconfig="${KUBECONFIG:+-kubeconfig=$KUBECONFIG}"
arg_kubectl="${KUBECTL:+-kubectl-path=$KUBECTL}"
arg_oc="${KUBECTL:+-oc-path=$KUBECTL}"
arg_gocli="${GOCLI:+-gocli-path=$GOCLI}"
arg_sc_snap="${SNAPSHOT_SC:+-snapshot-sc=$SNAPSHOT_SC}"
arg_sc_block="${BLOCK_SC:+-block-sc=$BLOCK_SC}"

test_args="${test_args} -ginkgo.v ${arg_master} ${arg_namespace} ${arg_kubeconfig} ${arg_kubectl} ${arg_oc} ${arg_gocli} ${arg_sc_snap} ${arg_sc_block}"

echo 'Wait until all CDI Pods are ready'
retry_counter=0
while [ $retry_counter -lt $MAX_CDI_WAIT_RETRY ] && [ -n "$(./cluster-up/kubectl.sh get pods -n $CDI_NAMESPACE -o'custom-columns=status:status.containerStatuses[*].ready' --no-headers | grep false)" ]; do
    retry_counter=$((retry_counter + 1))
    sleep $CDI_WAIT_TIME
    echo "Checking CDI pods again, count $retry_counter"
    if [ $retry_counter -gt 1 ] && [ "$((retry_counter % 6))" -eq 0 ]; then
        ./cluster-up/kubectl.sh get pods -n $CDI_NAMESPACE
    fi
done

if [ $retry_counter -eq $MAX_CDI_WAIT_RETRY ]; then
    echo "Not all CDI pods became ready"
    ./cluster-up/kubectl.sh get pods -n $CDI_NAMESPACE
    exit 1
fi

test_command="${TESTS_OUT_DIR}/tests.test -test.timeout 360m ${test_args}"
echo "$test_command"
(
    cd ${CDI_DIR}/tests
    ${test_command}
)
