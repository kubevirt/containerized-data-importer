#!/usr/bin/env bash

set -e

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh

FETCH_RPMS_IMAGE=${FETCH_RPMS_IMAGE:-kubevirt/fetch-rpms-workspace}
FEDORA_VERSION_RPMS=${FEDORA_VERSION_RPMS:-33}
BUILD_DIR="${CDI_DIR}/hack/build/docker/rpms"
# Build the container image 
docker build -t ${FETCH_RPMS_IMAGE} --build-arg "VERSION=${FEDORA_VERSION_RPMS}" -f ${BUILD_DIR}/Dockerfile  ${BUILD_DIR}

# Start rsync inside the container
ID=$(docker run -w /fetch --rm -td --security-opt label=disable --expose 873 -P ${FETCH_RPMS_IMAGE} /usr/bin/rsync --no-detach --daemon --verbose)
PORT=$(docker port $ID 873 | cut -d':' -f2)
# Copy the WORKSPACE inside the container
rsync -rlgoD ${CDI_DIR}/WORKSPACE rsync://root@127.0.0.1:${PORT}/build
# Run the script to update the rpms
docker exec -ti ${ID} /fetch/fetch-repo-workspace.py -i WORKSPACE -w
# Copy back the WORKSPACE
rsync -rlgoD  rsync://root@127.0.0.1:${PORT}/build/WORKSPACE ${CDI_DIR}/WORKSPACE
# Clean up
docker stop ${ID}

