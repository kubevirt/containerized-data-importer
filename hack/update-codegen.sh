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

export GO111MODULE=off

SCRIPT_ROOT="$(cd "$(dirname $0)/../" && pwd -P)"
CODEGEN_PKG=${CODEGEN_PKG:-$(
    cd ${SCRIPT_ROOT}
    ls -d -1 $GOPATH/src/k8s.io/code-generator 2>/dev/null || echo ../code-generator
)}

find "${SCRIPT_ROOT}/pkg/" -name "*generated*.go" -exec rm {} -f \;
rm -rf "${SCRIPT_ROOT}/pkg/client"

${SCRIPT_ROOT}/hack/build/build-go.sh generate

# generate the code with:
# --output-base    because this script should also be able to run inside the vendor dir of
#                  k8s.io/kubernetes. The output-base is needed for the generators to output into the vendor dir
#                  instead of the $GOPATH directly. For normal projects this can be dropped.
${CODEGEN_PKG}/generate-groups.sh "deepcopy,client,informer,lister" \
  kubevirt.io/containerized-data-importer/pkg/client kubevirt.io/containerized-data-importer/pkg/apis \
  "core:v1alpha1 upload:v1alpha1 core:v1beta1 upload:v1beta1" \
  --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

echo "Generating swagger doc"
swagger-doc -in ${SCRIPT_ROOT}/pkg/apis/core/v1alpha1/types.go
swagger-doc -in ${SCRIPT_ROOT}/pkg/apis/upload/v1alpha1/types.go

swagger-doc -in ${SCRIPT_ROOT}/pkg/apis/core/v1beta1/types.go
swagger-doc -in ${SCRIPT_ROOT}/pkg/apis/upload/v1beta1/types.go

echo "Generating openapi"
openapi-gen --input-dirs k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/api/core/v1,github.com/openshift/custom-resource-status/conditions/v1,kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1 \
    --output-package kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1 \
    --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

openapi-gen --input-dirs k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/api/core/v1,kubevirt.io/containerized-data-importer/pkg/apis/upload/v1alpha1 \
    --output-package kubevirt.io/containerized-data-importer/pkg/apis/upload/v1alpha1 \
    --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

openapi-gen --input-dirs k8s.io/apimachinery/pkg/api/resource,k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/apimachinery/pkg/runtime,k8s.io/api/core/v1,github.com/openshift/custom-resource-status/conditions/v1,kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1 \
    --output-package kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1 \
    --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

openapi-gen --input-dirs k8s.io/apimachinery/pkg/apis/meta/v1,k8s.io/api/core/v1,kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1 \
    --output-package kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1 \
    --go-header-file ${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt

(cd ${SCRIPT_ROOT}/tools/openapi-spec-generator/ && go build -o ../../bin/openapi-spec-generator)

${SCRIPT_ROOT}/bin/openapi-spec-generator > ${SCRIPT_ROOT}/api/openapi-spec/swagger.json

# the kubevirtci commit hash to vendor from
kubevirtci_git_hash=b98abc151e83a12f206d9c50bcf1989dd41b9c33

# remove previous cluster-up dir entirely before vendoring
rm -rf cluster-up

# download and extract the cluster-up dir from a specific hash in kubevirtci
curl -L https://github.com/kubevirt/kubevirtci/archive/${kubevirtci_git_hash}/kubevirtci.tar.gz | tar xz kubevirtci-${kubevirtci_git_hash}/cluster-up --strip-component 1
