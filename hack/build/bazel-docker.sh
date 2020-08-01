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
script_dir="$(readlink -f $(dirname $0))"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

WORK_DIR="/go/src/kubevirt.io/containerized-data-importer"
BUILDER_SPEC="${BUILD_DIR}/docker/builder"
BUILDER_VOLUME="kubevirt-cdi-volume"
BAZEL_BUILDER_SERVER="${BUILDER_VOLUME}-bazel-server"

# Be less verbose with bazel
if [ -n "${TRAVIS_JOB_ID}" ]; then
    cat >.bazelrc <<EOF
common --noshow_progress --noshow_loading_progress
EOF
fi

# Create the persistent docker volume
if [ -z "$(docker volume list | grep ${BUILDER_VOLUME})" ]; then
    if [ "$KUBEVIRTCI_RUNTIME" = "podman" ]; then
        docker volume create ${BUILDER_VOLUME}
    else
        docker volume create --name ${BUILDER_VOLUME}
    fi
fi

# Make sure that the output directory exists
echo "Making sure output directory exists..."
if [ "$KUBEVIRTCI_RUNTIME" = "podman" ]; then
    docker run -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable --rm --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} mkdir -p /root/go/src/kubevirt.io/containerized-data-importer/_out
else
    docker run -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label:disable --rm --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} mkdir -p /root/go/src/kubevirt.io/containerized-data-importer/_out
fi

echo "Starting rsyncd"
# Start an rsyncd instance and make sure it gets stopped after the script exits
if [ "$KUBEVIRTCI_RUNTIME" = "podman" ]; then
    RSYNC_CID_CDI=$(docker run -d -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable --expose 873 -P --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} /usr/bin/rsync --no-detach --daemon --verbose)
else
    RSYNC_CID_CDI=$(docker run -d -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label:disable --expose 873 -P --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} /usr/bin/rsync --no-detach --daemon --verbose)
fi

function finish() {
    docker stop ${RSYNC_CID_CDI} >/dev/null 2>&1 &
    docker rm -f ${RSYNC_CID_CDI} >/dev/null 2>&1 &
}
trap finish EXIT

if [ "$KUBEVIRTCI_RUNTIME" = "podman" ]; then
    RSYNCD_PORT=$(docker port $RSYNC_CID_CDI | cut -d':' -f2)
else
    RSYNCD_PORT=$(docker port $RSYNC_CID_CDI 873 | cut -d':' -f2)
fi

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
    --exclude "_out" \
    --exclude ".vagrant" \
    ${CDI_DIR}/ \
    "rsync://root@127.0.0.1:${RSYNCD_PORT}/build"


if [ "${KUBEVIRTCI_RUNTIME}" != "podman" ]; then
    volumes="-v ${BUILDER_VOLUME}:/root:rw,z"
    # append .docker directory as volume
    mkdir -p "${HOME}/.docker"
    volumes="$volumes -v ${HOME}/.docker:/root/.docker:ro,z"
else
    volumes="-v ${BUILDER_VOLUME}:/root:rw,z,exec"
fi

# Ensure that a bazel server is running
if [ -z "$(docker ps --format '{{.Names}}' | grep ${BAZEL_BUILDER_SERVER})" ]; then
    if [ "$KUBEVIRTCI_RUNTIME" = "podman" ]; then
        docker run --network host -d ${volumes} --security-opt label=disable --name ${BAZEL_BUILDER_SERVER} -w "/root/go/src/kubevirt.io/containerized-data-importer" --rm ${BUILDER_IMAGE} hack/build/bazel-server.sh
    else
        docker run --network host -d ${volumes} --security-opt label:disable --name ${BAZEL_BUILDER_SERVER} -w "/root/go/src/kubevirt.io/containerized-data-importer" --rm ${BUILDER_IMAGE} hack/build/bazel-server.sh
    fi
fi

echo "Starting bazel server"
# Run the command
test -t 1 && USE_TTY="-it"
docker exec ${USE_TTY} ${BAZEL_BUILDER_SERVER} /entrypoint-bazel.sh "$@"

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

_rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/build/manifests/generated/" "${CDI_DIR}/manifests/generated"

if [ "$SYNC_VENDOR" = "true" ] && [ -n $VENDOR_DIR ]; then
    _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/vendor" "${VENDOR_DIR}/"
fi
