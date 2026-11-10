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
    BUILD_BASE_ARGS=()
    PREFLIGHT=0
    case "${BUILDAH_PLATFORM_FLAG}," in
        *linux/s390x,*) [[ "${HOST_ARCHITECTURE}" = "s390x" ]] || PREFLIGHT=1 ;;
        *) ;;
    esac

    # Check s390x emulation before building the full image.
    if [[ "${PREFLIGHT}" = "1" ]]; then
        QEMU_STATIC_IMAGE="quay.io/linuxserver.io/qemu-static"
        BINFMT_DIR="/proc/sys/fs/binfmt_misc"
        PROBE_DIR=""
        PROBE_ID=""
        BUILDER_BASE_REF=""

        cleanup_probe() {
            local arch
            if [[ -n "${PROBE_ID}" ]]; then
                for arch in s390x arm64; do
                    buildah rmi "localhost/${PROBE_ID}-${arch}:probe" >/dev/null 2>&1 || true
                done
            fi
            rm -rf -- "${PROBE_DIR}" || true
        }

        pull_base() {
            buildah pull --platform "$1" "${BUILDER_BASE_REF}"
        }

        probe_platform() {
            buildah build --platform "$1" --pull=never --no-cache --build-arg BUILDER_BASE="${BUILDER_BASE_REF}" \
                -t "localhost/${PROBE_ID}-${1#linux/}:probe" -f "${PROBE_DIR}/Containerfile" "${PROBE_DIR}"
        }

        dump_binfmt() {
            local entry
            echo "binfmt_misc dump ($1):"
            for entry in "${BINFMT_DIR}/status" "${BINFMT_DIR}"/qemu-*; do
                if [[ -f "${entry}" ]]; then
                    echo "binfmt registration: ${entry}"
                    cat "${entry}" || true
                fi
            done
        }

        print_diagnostics() {
            local dmesg_out
            if [[ ! -f "${BINFMT_DIR}/register" ]]; then
                mount -t binfmt_misc binfmt_misc "${BINFMT_DIR}" \
                    || echo "WARNING: unable to mount binfmt_misc; handler dump will be empty" >&2
            fi
            uname -a || true
            rpm -q qemu-user-static-aarch64 qemu-user-static-s390x buildah podman crun runc || true
            dump_binfmt "before reset"
            if dmesg_out=$(dmesg 2>&1); then
                printf '%s\n' "${dmesg_out}" | grep -iE 'segfault|qemu' | tail -n 50 || true
            else
                echo "dmesg is not readable; kernel segfault messages are unavailable"
            fi
        }

        prepare_preflight() {
            local base_tag
            local digest
            PROBE_DIR=$(mktemp -d "${TMPDIR:-/tmp}/cdi-builder-probe-XXXXXX") \
                || { echo "WARNING: cannot create a temporary directory" >&2; return 1; }
            trap cleanup_probe EXIT
            PROBE_ID="${PROBE_DIR##*/}"
            PROBE_ID="${PROBE_ID,,}"
            base_tag=$(sed -n 's/^ARG BUILDER_BASE=//p' "${BUILDER_SPEC}/Dockerfile" | head -n 1 | tr -d '[:space:]') \
                || { echo "WARNING: cannot read ${BUILDER_SPEC}/Dockerfile" >&2; return 1; }
            if [[ "${base_tag##*/}" != *:* ]]; then
                echo "WARNING: no tagged 'ARG BUILDER_BASE=' in ${BUILDER_SPEC}/Dockerfile" >&2
                return 1
            fi
            skopeo inspect --raw "docker://${base_tag}" > "${PROBE_DIR}/base.json" \
                || { echo "WARNING: cannot read the manifest of ${base_tag}" >&2; return 1; }
            jq -e '.manifests | length > 0' "${PROBE_DIR}/base.json" >/dev/null \
                || { echo "WARNING: ${base_tag} is not a multi-arch manifest list" >&2; return 1; }
            digest=$(sha256sum "${PROBE_DIR}/base.json" | cut -d' ' -f1) \
                || { echo "WARNING: cannot compute the digest of ${base_tag}" >&2; return 1; }
            if [[ ! "${digest}" =~ ^[0-9a-f]{64}$ ]]; then
                echo "WARNING: cannot compute the digest of ${base_tag}" >&2
                return 1
            fi
            BUILDER_BASE_REF="${base_tag%:*}@sha256:${digest}"
            echo "Builder base: ${BUILDER_BASE_REF}"
            # shellcheck disable=SC2016
            printf '%s\n' 'ARG BUILDER_BASE' 'FROM ${BUILDER_BASE}' 'RUN uname -m && dnf --version' > "${PROBE_DIR}/Containerfile" \
                || { echo "WARNING: cannot write the probe Containerfile" >&2; return 1; }
            pull_base linux/s390x \
                || { echo "WARNING: unable to pull ${BUILDER_BASE_REF} for linux/s390x" >&2; return 1; }
        }

        run_preflight() {
            local rc=0
            local first_rc=0
            local t0
            local t1
            local plat
            local post_plats=(linux/s390x)

            t0=$(date -u +%Y-%m-%dT%H:%M:%SZ) || true
            probe_platform linux/s390x || first_rc=$?
            t1=$(date -u +%Y-%m-%dT%H:%M:%SZ) || true
            if [[ "${first_rc}" -eq 0 ]]; then
                echo "Preflight: linux/s390x probe passed"
                return 0
            fi
            echo "ERROR: preflight probe failed: platform=linux/s390x start=${t0} end=${t1} exit=${first_rc}" >&2
            print_diagnostics

            if [[ -z "${CDI_CRI}" ]]; then
                echo "ERROR: no container runtime to re-register QEMU" >&2
                return "${first_rc}"
            fi
            "${CDI_CRI}" run --rm --entrypoint /qemu/qemu-s390x "${QEMU_STATIC_IMAGE}" --version \
                || echo "WARNING: cannot print the QEMU version; trying the registration anyway" >&2
            # Reset replaces all visible qemu-* binfmt handlers, including arm64.
            "${CDI_CRI}" run --rm --privileged "${QEMU_STATIC_IMAGE}" --reset -p yes || rc=$?
            if [[ "${rc}" -ne 0 ]]; then
                echo "ERROR: QEMU registration failed (exit ${rc}); handlers may be partially modified" >&2
                return "${rc}"
            fi
            dump_binfmt "after reset"

            case "${BUILDAH_PLATFORM_FLAG}," in
                *linux/arm64,*)
                    case "${HOST_ARCHITECTURE}" in
                        aarch64|arm64) ;;
                        *) post_plats+=(linux/arm64) ;;
                    esac ;;
                *) ;;
            esac
            for plat in "${post_plats[@]}"; do
                rc=0
                pull_base "${plat}" || rc=$?
                if [[ "${rc}" -ne 0 ]]; then
                    echo "ERROR: unable to pull ${BUILDER_BASE_REF} for ${plat} after re-registration (exit ${rc})" >&2
                    return "${rc}"
                fi
                probe_platform "${plat}" || rc=$?
                if [[ "${rc}" -ne 0 ]]; then
                    echo "ERROR: probe after QEMU re-registration failed: ${plat} exit=${rc}" >&2
                    return "${rc}"
                fi
            done
            echo "Preflight: QEMU recovery succeeded (original linux/s390x probe exit=${first_rc}, ${t0}..${t1}); probes passed: ${post_plats[*]}"
        }

        if prepare_preflight; then
            preflight_rc=0
            run_preflight || preflight_rc=$?
            if [[ "${preflight_rc}" -ne 0 ]]; then
                exit "${preflight_rc}"
            fi
            BUILD_BASE_ARGS=(--build-arg "BUILDER_BASE=${BUILDER_BASE_REF}")
        else
            echo "WARNING: preflight skipped; building with the default base" >&2
        fi
    fi

    (cd ${BUILDER_SPEC} && buildah build ${BUILDAH_PLATFORM_FLAG} --build-arg ARCH=${ARCH} "${BUILD_BASE_ARGS[@]}" --manifest ${BUILDER_MANIFEST} .)
    buildah manifest push --all ${BUILDER_MANIFEST} docker://${BUILDER_MANIFEST}
    DIGEST=$(podman inspect $(podman images | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $3 }') | jq '.[]["Digest"]')
else
    (cd ${BUILDER_SPEC} && docker build --build-arg ARCH=${ARCH} --tag ${BUILDER_MANIFEST} .)
    docker push ${BUILDER_MANIFEST}
    DIGEST=$(docker images --digests | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $4 }')
fi

echo "Image: ${BUILDER_MANIFEST}"
echo "Digest: ${DIGEST}"
