#!/usr/bin/env bash

#Copyright 2018 The CDI Authors.
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

mkdir -p "${CDI_DIR}/_out"

BUILDER_SPEC="${BUILD_DIR}/docker/builder"
BUILDER_VOLUME="kubevirt-cdi-volume"
BAZEL_BUILDER_SERVER="${BUILDER_VOLUME}-bazel-server"
DOCKER_CA_CERT_FILE="${DOCKER_CA_CERT_FILE:-}"
DOCKERIZED_CUSTOM_CA_PATH="/etc/pki/ca-trust/source/anchors/custom-ca.crt"

DISABLE_SECCOMP=${DISABLE_SECCOMP:-}

SYNC_OUT=${SYNC_OUT:-true}
SYNC_VENDOR=${SYNC_VENDOR:-true}

# Be less verbose with bazel
if [ -n "${TRAVIS_JOB_ID}" ]; then
    cat >.bazelrc <<EOF
common --noshow_progress --noshow_loading_progress
EOF
fi

# Create the persistent docker volume
if [ -z "$(${CDI_CRI} volume list | grep ${BUILDER_VOLUME})" ]; then
    ${CDI_CRI} volume create ${BUILDER_VOLUME}
fi

# Make sure that the output directory exists
echo "Making sure output directory exists..."
${CDI_CRI} run -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable $DISABLE_SECCOMP --rm --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} mkdir -p /root/go/src/kubevirt.io/containerized-data-importer/_out

echo "Starting rsyncd"
# Start an rsyncd instance and make sure it gets stopped after the script exits
RSYNC_CID_CDI=$(${CDI_CRI} run -d -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable $DISABLE_SECCOMP --expose 873 -P --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} /usr/bin/rsync --no-detach --daemon --verbose)

function finish() {
    ${CDI_CRI} stop --time 1 ${RSYNC_CID_CDI} >/dev/null 2>&1
    ${CDI_CRI} rm -f ${RSYNC_CID_CDI} >/dev/null 2>&1
}
trap finish EXIT

RSYNCD_PORT=$(${CDI_CRI} port $RSYNC_CID_CDI | cut -d':' -f2)

rsynch_fail_count=0

while ! rsync ${CDI_DIR}/${RSYNCTEMP} "rsync://root@127.0.0.1:${RSYNCD_PORT}/build/${RSYNCTEMP}" &>/dev/null; do
    if [[ "$rsynch_fail_count" -eq 0 ]]; then
        printf "Waiting for rsyncd to be ready"
        sleep .1
    elif [[ "$rsynch_fail_count" -lt 30 ]]; then
        printf "."
        sleep 1
    else
        printf "failed"
        break
    fi
    rsynch_fail_count=$((rsynch_fail_count + 1))
done

printf "\n"

rsynch_fail_count=0

_rsync() {
    rsync -al "$@"
}

echo "Rsyncing ${CDI_DIR} to container"
# Copy CDI into the persistent docker volume
_rsync \
    --delete \
    --exclude 'bazel-bin' \
    --exclude 'bazel-genfiles' \
    --exclude 'bazel-containerized-data-importer' \
    --exclude 'bazel-out' \
    --exclude 'bazel-testlogs' \
    --exclude 'cluster-up/cluster/**/.kubectl' \
    --exclude 'cluster-up/cluster/**/.oc' \
    --exclude 'cluster-up/cluster/**/.kubeconfig' \
    --exclude ".vagrant" \
    ${CDI_DIR}/ \
    "rsync://root@127.0.0.1:${RSYNCD_PORT}/build"

if [ "${CDI_CRI}" = "docker" ]; then
    volumes="-v ${BUILDER_VOLUME}:/root:rw,z"
    # append .docker directory as volume
    mkdir -p "${HOME}/.docker"
    volumes="$volumes -v ${HOME}/.docker:/root/.docker:ro,z"
else
    volumes="-v ${BUILDER_VOLUME}:/root:rw,z,exec"
fi

if [ "${CI}" = "true" ]; then
    mkdir -p "$HOME/containers"
    volumes="$volumes -v ${HOME}/containers:/root/containers:ro,z"
fi

if [ -n "$DOCKER_CA_CERT_FILE" ]; then
    volumes="$volumes -v ${DOCKER_CA_CERT_FILE}:${DOCKERIZED_CUSTOM_CA_PATH}:ro,z"
fi

# Ensure that a bazel server is running
if [ -z "$(${CDI_CRI} ps --format '{{.Names}}' | grep ${BAZEL_BUILDER_SERVER})" ]; then
    ${CDI_CRI} run --ulimit nofile=10000:10000 $DISABLE_SECCOMP --network host -d ${volumes} --security-opt label=disable --name ${BAZEL_BUILDER_SERVER} -e "GOPATH=/root/go" -w "/root/go/src/kubevirt.io/containerized-data-importer" --rm ${BUILDER_IMAGE} hack/build/bazel-server.sh
fi

echo "Starting bazel server"
# Run the command
test -t 1 && USE_TTY="-it"
${CDI_CRI} exec ${USE_TTY} ${BAZEL_BUILDER_SERVER} /entrypoint-bazel.sh "$@"

# Copy the whole containerized-data-importer data out to get generated sources and formatting changes
_rsync \
    --exclude 'bazel-bin' \
    --exclude 'bazel-genfiles' \
    --exclude 'bazel-containerized-data-importer' \
    --exclude 'bazel-out' \
    --exclude 'bazel-testlogs' \
    --exclude 'cluster-up/cluster/**/.kubectl' \
    --exclude 'cluster-up/cluster/**/.oc' \
    --exclude 'cluster-up/cluster/**/.kubeconfig' \
    --exclude "_out" \
    --exclude "vendor" \
    --exclude ".vagrant" \
    --exclude ".git" \
    "rsync://root@127.0.0.1:${RSYNCD_PORT}/build" \
    ${CDI_DIR}/

if [ "$SYNC_VENDOR" = "true" ] && [ -n $VENDOR_DIR ]; then
    _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/vendor" "${VENDOR_DIR}/"
fi

# Copy the build output out of the container, make sure that _out exactly matches the build result
if [ "$SYNC_OUT" = "true" ]; then
    _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/out" ${OUT_DIR}
fi
