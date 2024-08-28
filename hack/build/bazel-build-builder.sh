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

set -ex
script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

if [ "${CDI_CONTAINER_BUILDCMD}" = "buildah" ]; then
    BUILDAH_PLATFORM_FLAG="--platform linux/amd64"
    
    if rpm -qa | grep -q qemu-user-static-aarch64; then
        BUILDAH_PLATFORM_FLAG="${BUILDAH_PLATFORM_FLAG},linux/arm64"
    fi
    
    if rpm -qa | grep -q qemu-user-static-s390x; then
        BUILDAH_PLATFORM_FLAG="${BUILDAH_PLATFORM_FLAG},linux/s390x"
    fi
    
    echo "Building with $BUILDAH_PLATFORM_FLAG"
fi

# The other circumstance in which we need to build the builder image is
# in the course of test and development of the builder image itself.
# we'll signal this circumstance by setting the env variable ADHOC_BUILDER
#
# also instead of hard-coding the UNTAGGED_BUILDER_IMAGE, we're going
# to use DOCKER_PREFIX as it is set in config.sh and used elsewhere in
# the build system, to introduce a little more consistency

if [ x"${ADHOC_BUILDER}" != "x" ]; then
    BUILDER_SPEC="${BUILD_DIR}/docker/builder"
    UNTAGGED_BUILDER_IMAGE="${DOCKER_PREFIX}/kubevirt-cdi-bazel-builder"
    BUILDER_TAG=$(date +"%y%m%d%H%M")-$(git rev-parse --short HEAD)
    BUILDER_MANIFEST=${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
    echo "export BUILDER_IMAGE=$BUILDER_MANIFEST"

    #Build the encapsulated compile and test container
    if [ "${CDI_CONTAINER_BUILDCMD}" = "buildah" ]; then
        (cd ${BUILDER_SPEC} && buildah build ${BUILDAH_PLATFORM_FLAG} --build-arg ARCH=${ARCH} --manifest ${BUILDER_MANIFEST} .)
        buildah manifest push --all ${BUILDER_MANIFEST} docker://${BUILDER_MANIFEST}
        DIGEST=$(podman inspect $(podman images | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $3 }') | jq '.[]["Digest"]')
    else
        (cd ${BUILDER_SPEC} && docker build --build-arg ARCH=${ARCH} --tag ${BUILDER_MANIFEST} .)
        docker push ${BUILDER_MANIFEST}
        DIGEST=$(docker images --digests | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $4 }')
    fi

    echo "Image: ${BUILDER_MANIFEST}"
    echo "Digest: ${DIGEST}"
fi
