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
set -x
export GO111MODULE=on

export SCRIPT_ROOT="$(cd "$(dirname $0)/../" && pwd -P)"
CODEGEN_PKG=${CODEGEN_PKG:-$(
    cd ${SCRIPT_ROOT}
    ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator
)}
OPENAPI_PKG=${OPENAPI_PKG:-$(
    cd ${SCRIPT_ROOT}
    ls -d -1 ./vendor/k8s.io/kube-openapi 2>/dev/null || echo ../kube-openapi
)}

(GOPROXY=off go install ${CODEGEN_PKG}/cmd/deepcopy-gen)
(GOPROXY=off go install ${CODEGEN_PKG}/cmd/client-gen)
(GOPROXY=off go install ${CODEGEN_PKG}/cmd/informer-gen)
(GOPROXY=off go install ${CODEGEN_PKG}/cmd/lister-gen)
(GOPROXY=off go install ${OPENAPI_PKG}/cmd/openapi-gen)

find "${SCRIPT_ROOT}/pkg/" -name "*generated*.go" -exec rm {} -f \;
find "${SCRIPT_ROOT}/staging/src/kubevirt.io/containerized-data-importer-api/" -name "*generated*.go" -exec rm {} -f \;
rm -rf "${SCRIPT_ROOT}/pkg/client"
mkdir "${SCRIPT_ROOT}/pkg/client"
mkdir "${SCRIPT_ROOT}/pkg/client/clientset"
mkdir "${SCRIPT_ROOT}/pkg/client/informers"
mkdir "${SCRIPT_ROOT}/pkg/client/listers"

${SCRIPT_ROOT}/hack/build/build-go.sh generate

deepcopy-gen \
    --output-file zz_generated.deepcopy.go \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1alpha1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1

client-gen \
    --clientset-name versioned \
    --input-base kubevirt.io/containerized-data-importer-api/pkg/apis \
    --output-dir "${SCRIPT_ROOT}/pkg/client/clientset" \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/client/clientset \
    --apply-configuration-package '' \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    --input core/v1alpha1 \
    --input core/v1beta1 \
    --input upload/v1beta1 \
    --input forklift/v1beta1

lister-gen \
    --output-dir "${SCRIPT_ROOT}/pkg/client/listers" \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/client/listers \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1alpha1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1

informer-gen \
    --versioned-clientset-package kubevirt.io/containerized-data-importer/pkg/client/clientset/versioned \
    --listers-package kubevirt.io/containerized-data-importer/pkg/client/listers \
    --output-dir "${SCRIPT_ROOT}/pkg/client/informers" \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/client/informers \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1alpha1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1 \
    kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1

echo "Generating swagger doc"
swagger-doc -in ${SCRIPT_ROOT}/staging/src/kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1alpha1/types.go

swagger-doc -in ${SCRIPT_ROOT}/staging/src/kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1/types.go
swagger-doc -in ${SCRIPT_ROOT}/staging/src/kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1/types.go

swagger-doc -in ${SCRIPT_ROOT}/staging/src/kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1/types.go

echo "Generating openapi"
openapi-gen \
    --output-dir ${SCRIPT_ROOT}/pkg/apis/core/v1alpha1 \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/apis/core/v1alpha1 \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    --output-file openapi_generated.go \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1alpha1 \
    k8s.io/apimachinery/pkg/api/resource \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    k8s.io/apimachinery/pkg/runtime \
    k8s.io/api/core/v1 \
    github.com/openshift/custom-resource-status/conditions/v1 \
    kubevirt.io/controller-lifecycle-operator-sdk/api

openapi-gen \
    --output-dir ${SCRIPT_ROOT}/pkg/apis/core/v1beta1 \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/apis/core/v1beta1 \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    --output-file openapi_generated.go \
    kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1 \
    k8s.io/apimachinery/pkg/api/resource \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    k8s.io/apimachinery/pkg/runtime \
    k8s.io/api/core/v1 \
    github.com/openshift/custom-resource-status/conditions/v1 \
    kubevirt.io/controller-lifecycle-operator-sdk/api

openapi-gen \
    --output-dir ${SCRIPT_ROOT}/pkg/apis/upload/v1beta1 \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/apis/upload/v1beta1 \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    --output-file openapi_generated.go \
    kubevirt.io/containerized-data-importer-api/pkg/apis/upload/v1beta1 \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    k8s.io/api/core/v1

openapi-gen \
    --output-dir ${SCRIPT_ROOT}/pkg/apis/forklift/v1beta1 \
    --output-pkg kubevirt.io/containerized-data-importer/pkg/apis/forklift/v1beta1 \
    --go-header-file "${SCRIPT_ROOT}/hack/custom-boilerplate.go.txt" \
    --output-file openapi_generated.go \
    kubevirt.io/containerized-data-importer-api/pkg/apis/forklift/v1beta1 \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    k8s.io/api/core/v1

(cd ${SCRIPT_ROOT}/tools/openapi-spec-generator/ && go build -o ../../bin/openapi-spec-generator)

${SCRIPT_ROOT}/bin/openapi-spec-generator >${SCRIPT_ROOT}/api/openapi-spec/swagger.json

echo "************* running controller-gen to generate schema yaml ********************"
(
    mkdir -p "${SCRIPT_ROOT}/_out/manifests/schema"
    find "${SCRIPT_ROOT}/_out/manifests/schema/" -type f -exec rm {} -f \;
    cd ./staging/src/kubevirt.io/containerized-data-importer-api
    controller-gen crd:crdVersions=v1 output:dir=${SCRIPT_ROOT}/_out/manifests/schema paths=./pkg/apis/core/...
    controller-gen crd:crdVersions=v1 output:dir=${SCRIPT_ROOT}/_out/manifests/schema paths=./pkg/apis/forklift/...

)
(cd "${SCRIPT_ROOT}/tools/crd-generator/" && go build -o "${SCRIPT_ROOT}/bin/crd-generator" ./...)

${SCRIPT_ROOT}/bin/crd-generator --crdDir=${SCRIPT_ROOT}/_out/manifests/schema/ --outputDir=${SCRIPT_ROOT}/pkg/operator/resources/
