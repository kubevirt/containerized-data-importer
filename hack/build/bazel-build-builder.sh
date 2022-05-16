#!/usr/bin/env bash

#Copyright 2019 The CDI Authors.
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

set -e
script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

if [ "${CDI_CONTAINER_BUILDCMD}" = "$(which buildah)" ]; then
    if [ -e /proc/sys/fs/binfmt_misc/qemu-aarch64 ]; then
        BUILDAH_PLATFORM_FLAG="--platform linux/amd64,linux/arm64"
    else
        if [ "$(uname)" = "Linux" ]; then
            echo "Expecting qemu-user-static to be installed on the host machine, please install it"
            exit 1
        fi
        BUILDAH_PLATFORM_FLAG=""
    fi
fi

if ! git diff-index --quiet HEAD~1 hack/build/docker; then
    #Since this only runs during the post-submit job, the PR will have squashed into a single
    #commit and we can use HEAD~1 to compare.
    BUILDER_SPEC="${BUILD_DIR}/docker/builder"
    UNTAGGED_BUILDER_IMAGE=quay.io/kubevirt/kubevirt-cdi-bazel-builder
    BUILDER_TAG=$(date +"%y%m%d%H%M")-$(git rev-parse --short HEAD)
    echo "$DOCKER_PREFIX:$DOCKER_TAG"

    #Build the encapsulated compile and test container
    if [ "${CDI_CONTAINER_BUILDCMD}" = "$(which buildah)" ]; then
        (cd ${BUILDER_SPEC} && buildah build ${BUILDAH_PLATFORM_FLAG} --manifest ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} .)
    else
        (cd ${BUILDER_SPEC} && docker build ${BUILDAH_PLATFORM_FLAG} --tag ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} .)
    fi

    DIGEST=$(docker images --digests | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $4 }')
    echo "Image: ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}"
    echo "Digest: ${DIGEST}"

    if [ "${CDI_CONTAINER_BUILDCMD}" = "$(which buildah)" ]; then
        buildah manifest push --all ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} docker://${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
    else
        docker push ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
    fi
fi
