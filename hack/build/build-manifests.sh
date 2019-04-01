#!/usr/bin/env bash

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

set -euo pipefail

script_dir="$(readlink -f $(dirname $0))"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

generator="${BIN_DIR}/manifest-generator"

(cd "${CDI_DIR}/tools/manifest-generator/" && go build -o "${generator}" ./...)


source "${script_dir}"/resource-generator.sh


mkdir -p "${MANIFEST_GENERATED_DIR}/"
#generate controller related manifests used to deploy cdi without operator
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "controller-rbac" "cdi-controller.k8s.rbac.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "apiserver-rbac" "cdi-apiserver.k8s.rbac.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "crd-resources" "cdi-resources.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "apiserver" "cdi-apiserver.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "controller" "cdi-controller.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "uploadproxy" "cdi-uploadproxy.yaml" 


#generate operator related manifests used to deploy cdi with operator-framework
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "operator-rbac" "rbac-operator.authorization.k8s.yaml.in" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "operator-deployment" "cdi-operator-deployment.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "operator-cdi-crd" "cdi-crd.yaml" 
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "operator-configmap-cr" "cdi-configmap-cr.yaml" 

#process templated manifests and populate them with generated manifests
tempDir=${MANIFEST_TEMPLATE_DIR}
processDirTemplates ${tempDir} ${OUT_DIR}/manifests ${OUT_DIR}/manifests/templates ${generator} ${MANIFEST_GENERATED_DIR} 
processDirTemplates ${tempDir}/release ${OUT_DIR}/manifests/release ${OUT_DIR}/manifests/templates/release ${generator} ${MANIFEST_GENERATED_DIR}
processDirTemplates ${tempDir}/release/olm ${OUT_DIR}/manifests/release/olm ${OUT_DIR}/manifests/templates/release/olm ${generator} ${MANIFEST_GENERATED_DIR}
processDirTemplates ${tempDir}/release/olm/bundle ${OUT_DIR}/manifests/release/olm/bundle ${OUT_DIR}/manifests/templates/release/olm/bundle ${generator} ${MANIFEST_GENERATED_DIR}


