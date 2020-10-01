#!/usr/bin/env bash

# Copyright 2017 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE}")/..

DIFFROOT="${SCRIPT_ROOT}/pkg"
TMP_DIFFROOT="${SCRIPT_ROOT}/_tmp/pkg"
_tmp="${SCRIPT_ROOT}/_tmp"

cleanup() {
    rm -rf "${_tmp}"
}
trap "cleanup" EXIT SIGINT

cleanup

mkdir -p "${TMP_DIFFROOT}"
cp -a "${DIFFROOT}"/* "${TMP_DIFFROOT}"

"${SCRIPT_ROOT}/hack/update-codegen.sh"
echo "diffing ${DIFFROOT} against freshly generated codegen"
ret=0
diff -Naupr "${DIFFROOT}" "${TMP_DIFFROOT}" -x *.bazel || ret=$?
cp -a "${TMP_DIFFROOT}"/* "${DIFFROOT}"
if [[ $ret -eq 0 ]]; then
    echo "${DIFFROOT} up to date."
else
    echo "${DIFFROOT} is out of date. Please run hack/update-codegen.sh"
    exit 1
fi

echo "************** verifying schemas match *****************"
go build -o ./bin/schema-exporter ./tools/schema-exporter
./bin/schema-exporter -export-path ./_out/manifests/code_schema

curl https://github.com/mikefarah/yq/releases/download/3.3.2/yq_linux_amd64 -L --output ./bin/yq
chmod +x ./bin/yq
./bin/yq compare _out/manifests/schema/cdi.kubevirt.io_cdiconfigs.yaml _out/manifests/code_schema/cdiconfigs.cdi.kubevirt.io spec || (echo "CDIConfg crd schema does not match" && exit 1)
./bin/yq compare _out/manifests/schema/cdi.kubevirt.io_cdis.yaml _out/manifests/code_schema/cdis.cdi.kubevirt.io spec || (echo "CDI crd schema does not match" && exit 1)
./bin/yq compare _out/manifests/schema/cdi.kubevirt.io_datavolumes.yaml _out/manifests/code_schema/datavolumes.cdi.kubevirt.io spec || (echo "Datavolume crd schema does not match" && exit 1)