#!/usr/bin/env bash
set -e

source hack/build/common.sh
source hack/build/config.sh


docker_opt="${1:-build}"
shift
targets="${@:-${DOCKER_IMAGES}}"
tmp_dir="tmp"

printf "Building targets: %s\n" "${targets}"

if [ "${docker_opt}" == "build" ]; then
    for tgt in ${targets}; do
        BIN_NAME="$(basename ${tgt})"
        (
            cd "${CMD_OUT_DIR}/${BIN_NAME}"
            docker "${docker_opt}" -t ${DOCKER_REPO}/${BIN_NAME}:${DOCKER_TAG} .
        )
    done
elif [ "${docker_opt}" == "push" ]; then
    for tgt in ${targets}; do
        docker ${docker_opt} ${DOCKER_REPO}/${tgt}:${DOCKER_TAG}
    done
fi
