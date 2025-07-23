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

set -exuo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

generator="${BIN_DIR}/manifest-generator"

(cd "${CDI_DIR}/tools/manifest-generator/" && GO111MODULE=${GO111MODULE:-off} go build -o "${generator}" ./...)

echo "DOCKER_PREFIX=${DOCKER_PREFIX}"
echo "DOCKER_TAG=${DOCKER_TAG}"
echo "VERBOSITY=${VERBOSITY}"
echo "PULL_POLICY=${PULL_POLICY}"
echo "CDI_NAMESPACE=${CDI_NAMESPACE}"

source "${script_dir}"/resource-generator.sh

mkdir -p "${MANIFEST_GENERATED_DIR}/"

# Update manifests to replace extra images with internal registry ones
if [ -n "${EXTRA_IMAGES-}" ]; then
    for img in ${EXTRA_IMAGES}; do
        base_image_path="${img#*/}"
        local_img_url="registry:5000/${base_image_path}"
        if [ "$img" = "${EXTERNAL_IMAGE_MINIO}" ]; then
            EXTERNAL_IMAGE_MINIO=$local_img_url
        fi
        if [ "$img" = "${EXTERNAL_IMAGE_FAKEOVIRT}" ]; then
            EXTERNAL_IMAGE_FAKEOVIRT=$local_img_url
        fi
    done
fi

#generate operator related manifests used to deploy cdi with operator-framework
generateResourceManifest $generator $MANIFEST_GENERATED_DIR "operator" "everything" "operator-everything.yaml.in"

#process templated manifests and populate them with generated manifests
tempDir=${MANIFEST_TEMPLATE_DIR}
processDirTemplates ${tempDir} ${OUT_DIR}/manifests ${OUT_DIR}/manifests/templates ${generator} ${MANIFEST_GENERATED_DIR}
processDirTemplates ${tempDir}/release ${OUT_DIR}/manifests/release ${OUT_DIR}/manifests/templates/release ${generator} ${MANIFEST_GENERATED_DIR}

testsManifestsDir=${CDI_DIR}/tests/manifests
processDirTemplates ${testsManifestsDir}/templates ${testsManifestsDir}/out ${testsManifestsDir}/out/templates ${generator} ${MANIFEST_GENERATED_DIR}
