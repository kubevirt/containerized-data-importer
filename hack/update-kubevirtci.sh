#!/bin/sh
#
# Copyright 2021 The CDI Authors.
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

SCRIPT_ROOT="$(cd "$(dirname $0)/../" && pwd -P)"

# the kubevirtci commit hash to vendor from
kubevirtci_git_hash=0963b51c10b09220579be36fe9c0dcee74a0b865

# remove previous cluster-up dir entirely before vendoring
rm -rf ${SCRIPT_ROOT}/cluster-up

# download and extract the cluster-up dir from a specific hash in kubevirtci
curl -L https://github.com/kubevirt/kubevirtci/archive/${kubevirtci_git_hash}/kubevirtci.tar.gz | tar xz kubevirtci-${kubevirtci_git_hash}/cluster-up --strip-component 1

rm -f "${SCRIPT_ROOT}/cluster-up/cluster/kind-k8s-sriov-1.17.0/csrcreator/certsecret.go"

echo "KUBEVIRTCI_TAG=$(curl -L https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirtci/latest)" >>${SCRIPT_ROOT}/cluster-up/hack/common.sh
