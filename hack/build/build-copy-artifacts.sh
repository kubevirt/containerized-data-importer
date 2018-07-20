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

set -eo pipefail

script_dir="$(readlink -f $(dirname $0))"
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
        cp -f "${CDI_DIR}/cmd/${BIN_NAME}/script.sh" "${CMD_OUT_DIR}/${BIN_NAME}/"
    fi
    # Copy respective docker files to the directory of the build artifact
    cp -f "${BUILD_DIR}/docker/${BIN_NAME}/Dockerfile" "${CMD_OUT_DIR}/${BIN_NAME}/"

done
