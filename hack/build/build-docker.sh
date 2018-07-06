#!/usr/bin/env bash
set -e
echo $PWD

source hack/build/common.sh
source hack/build/config.sh


docker_opt="${1:-build}"
shift
targets="${@:-${DOCKER_IMAGES}}"
tmp_dir="tmp"

printf "Building targets: %s\n" "${targets}"

if [ "${docker_opt}" == "build" ]; then
    for tgt in ${targets}; do
        (
            cd "${BUILD_DIR}/docker/$tgt"

            # Handle cdi-cloner special case - it has no build artifact, just the script.sh
            if [ "${tgt}" == "${CLONER}" ]; then
                docker build -t ${DOCKER_REPO}/${tgt}:${DOCKER_TAG} .
            else
                rm -rf ${tmp_dir}
                mkdir -p ${tmp_dir}
                cp Dockerfile ${tmp_dir}/
                cp "${CMD_OUT_DIR}/${tgt}/${tgt}" ${tmp_dir}/
                cd ${tmp_dir}/
                docker build -t ${DOCKER_REPO}/${tgt}:${DOCKER_TAG} .
                rm -rf ${tmp_dir}
            fi
        )
    done
elif [ "${docker_opt}" == "push" ]; then
    for tgt in ${targets}; do
        docker ${docker_opt} ${DOCKER_REPO}/${tgt}:${DOCKER_TAG}
    done
fi
