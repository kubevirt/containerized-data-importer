#!/usr/bin/env bash

set -eo pipefail

script_dir="$(dirname ${BASH_SOURCE[0]})"

source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

targets="${@:-${DOCKER_IMAGES}}"

if [ -z "${targets}" ]; then
    targets=${DOCKER_IMAGES}
fi

for tgt in ${targets}; do
    BIN_NAME="$(basename ${tgt})"
    # Cloner has no build artifact, copy script.sh as well
    if [[ "${BIN_NAME}" == "${CLONER}" ]]; then
        mkdir -p "${CMD_OUT_DIR}/${BIN_NAME}/"
        cp -f "${CDI_DIR}"/cmd/"${BIN_NAME}"/script.sh "${CMD_OUT_DIR}"/"${BIN_NAME}"/
    fi
    # Copy respective docker files to the directory of the build artifact
    cp -f "${BUILD_DIR}"/docker/"${BIN_NAME}"/* "${CMD_OUT_DIR}"/"${BIN_NAME}"/

done
