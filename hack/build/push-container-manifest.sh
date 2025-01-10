#!/usr/bin/env bash
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
# Copyright 2023 NVIDIA CORPORATION
#

source hack/build/common.sh

# No need to push manifests if using a single arch
build_count=$(echo ${BUILD_ARCH//,/ } | wc -w)
if [ "$build_count" -lt 2 ]; then
    exit 0
fi

function podman_push_manifest() {
    image=$1
    # FIXME: Workaround https://github.com/containers/podman/issues/18360 and remove once https://github.com/containers/podman/commit/bab4217cd16be609ac35ccf3061d1e34f787856f is released
    echo ${CDI_CRI} manifest create ${DOCKER_PREFIX}/${image}:${DOCKER_TAG}
    ${CDI_CRI} manifest create ${DOCKER_PREFIX}/${image}:${DOCKER_TAG}
    for ARCH in ${BUILD_ARCH//,/ }; do
        FORMATED_ARCH=$(format_archname ${ARCH})
        if [ ! -f ${DIGESTS_DIR}/${FORMATED_ARCH}/bazel-bin/push-$image.digest ]; then
            continue
        fi
        digest=$(cat ${DIGESTS_DIR}/${FORMATED_ARCH}/bazel-bin/push-$image.digest)
        ${CDI_CRI} manifest add ${DOCKER_PREFIX}/${image}:${DOCKER_TAG} ${DOCKER_PREFIX}/${image}@${digest}
    done
    ${CDI_CRI} manifest push --all ${DOCKER_PREFIX}/${image}:${DOCKER_TAG} ${DOCKER_PREFIX}/${image}:${DOCKER_TAG}
}

function docker_push_manifest() {
    image=$1
    MANIFEST_IMAGES=""
    for ARCH in ${BUILD_ARCH//,/ }; do
        FORMATED_ARCH=$(format_archname ${ARCH})
        if [ ! -f ${DIGESTS_DIR}/${FORMATED_ARCH}/bazel-bin/push-$image.digest ]; then
            continue
        fi
        digest=$(cat ${DIGESTS_DIR}/${FORMATED_ARCH}/bazel-bin/push-$image.digest)
        MANIFEST_IMAGES="${MANIFEST_IMAGES} --amend ${DOCKER_PREFIX}/${image}@${digest}"
    done
    echo ${CDI_CRI} manifest create ${DOCKER_PREFIX}/${image}:${DOCKER_TAG} ${MANIFEST_IMAGES}
    ${CDI_CRI} manifest create ${DOCKER_PREFIX}/${image}:${DOCKER_TAG} ${MANIFEST_IMAGES}
    ${CDI_CRI} manifest push ${DOCKER_PREFIX}/${image}:${DOCKER_TAG}
}

export DOCKER_CLI_EXPERIMENTAL=enabled
for image in $(find ${DIGESTS_DIR}/*/bazel-bin/ -name '*.digest' -printf '%f\n' | sed s/^push-//g | sed s/\.digest$//g | sort -u); do
    if [ "${CDI_CRI}" = "podman" ]; then
        podman_push_manifest $image
    else
        docker_push_manifest $image
    fi
done
