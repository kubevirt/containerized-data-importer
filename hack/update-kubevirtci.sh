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

# the kubevirtci release to vendor from (https://github.com/kubevirt/kubevirtci/releases)
kubevirtci_release_tag=2108081530-91f55e3

# remove previous cluster-up dir entirely before vendoring
rm -rf ${SCRIPT_ROOT}/cluster-up

# download and extract the cluster-up dir from a specific hash in kubevirtci
curl -L https://github.com/kubevirt/kubevirtci/archive/${kubevirtci_release_tag}/kubevirtci.tar.gz | tar xz kubevirtci-${kubevirtci_release_tag}/cluster-up --strip-component 1

echo "KUBEVIRTCI_TAG=${kubevirtci_release_tag}" >>${SCRIPT_ROOT}/cluster-up/hack/common.sh
