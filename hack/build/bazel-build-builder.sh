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

    uname -a
    rpm -q qemu-user-static-aarch64 qemu-user-static-s390x buildah podman crun runc || true

    echo "Building with $BUILDAH_PLATFORM_FLAG"
    for binfmt_entry in /proc/sys/fs/binfmt_misc/*; do
        case "${binfmt_entry}" in
            */register|*/status) continue ;;
            *) ;;
        esac
        if [[ -f "${binfmt_entry}" ]]; then
            echo "binfmt registration: ${binfmt_entry}"
            cat "${binfmt_entry}" || echo "Unable to read binfmt registration: ${binfmt_entry}" >&2
        fi
    done
fi

# Instead of hard-coding the UNTAGGED_BUILDER_IMAGE, we're going
# to use DOCKER_PREFIX as it is set in config.sh and used elsewhere in
# the build system, to introduce a little more consistency

BUILDER_SPEC="${BUILD_DIR}/docker/builder"
UNTAGGED_BUILDER_IMAGE="${DOCKER_PREFIX}/kubevirt-cdi-bazel-builder"
BUILDER_TAG=$(date +"%y%m%d%H%M")-$(git rev-parse --short HEAD)
BUILDER_MANIFEST=${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
echo "export BUILDER_IMAGE=$BUILDER_MANIFEST"

#Build the encapsulated compile and test container
if [ "${CDI_CONTAINER_BUILDCMD}" = "buildah" ]; then
    case "${HOST_ARCHITECTURE}" in
        x86_64) host_platform=linux/amd64 ;;
        aarch64|arm64) host_platform=linux/arm64 ;;
        *) host_platform=linux/${HOST_ARCHITECTURE} ;;
    esac
    needs_emulation=0
    IFS=, read -r -a platforms <<< "${BUILDAH_PLATFORM_FLAG#--platform }"
    for platform in "${platforms[@]}"; do
        [[ "${platform}" = "${host_platform}" ]] || needs_emulation=1
    done

    if [[ "${needs_emulation}" = "1" ]]; then
        # Set up QEMU for foreign architectures using KubeVirt's registration command.
        # This replaces all visible qemu-* handlers.
        QEMU_STATIC_IMAGE="quay.io/linuxserver.io/qemu-static"
        if [[ -z "${CDI_CRI}" ]]; then
            echo "ERROR: no container runtime available to register QEMU interpreters" >&2
            exit 1
        fi
        qemu_rc=0
        "${CDI_CRI}" run --rm --privileged "${QEMU_STATIC_IMAGE}" --reset -p yes || qemu_rc=$?
        if [[ "${qemu_rc}" -ne 0 ]]; then
            echo "ERROR: QEMU registration failed (exit ${qemu_rc}); qemu-* handlers may be partially modified" >&2
            exit "${qemu_rc}"
        fi
        if [[ ! -f /proc/sys/fs/binfmt_misc/register ]]; then
            mount -t binfmt_misc binfmt_misc /proc/sys/fs/binfmt_misc \
                || echo "WARNING: unable to mount binfmt_misc; cannot show the registered handlers" >&2
        fi
        for handler in /proc/sys/fs/binfmt_misc/qemu-s390x /proc/sys/fs/binfmt_misc/qemu-aarch64; do
            if [[ -f "${handler}" ]]; then
                echo "binfmt registration after QEMU setup: ${handler}"
                cat "${handler}" || true
            fi
        done
    fi

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
