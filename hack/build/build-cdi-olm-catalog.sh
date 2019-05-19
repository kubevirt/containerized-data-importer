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

set -euo pipefail

script_dir="$(readlink -f $(dirname $0))"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh


OUT_PATH="${OUT_DIR}"
OLM_CATALOG_INIT_PATH="tools/${CDI_OLM_CATALOG}"
OLM_CATALOG_OUT_PATH=${OUT_PATH}/${OLM_CATALOG_INIT_PATH}
OLM_MANIFESTS_SRC_PATH=${OUT_PATH}/manifests/release/olm/bundle
OLM_MANIFESTS_DIR=olm-catalog
OLM_PACKAGE=${QUAY_REPOSITORY}

#create directory for olm catalog container
mkdir -p  "${OLM_CATALOG_OUT_PATH}"


#copy OLM manifests of a provided csv version to a dedicated directory
function packBundle {
    csv=$1
    mkdir -p ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE=}/${csv}
    cp ${OLM_MANIFESTS_SRC_PATH}/${csv} ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/${csv}
    cp ${OLM_MANIFESTS_SRC_PATH}/*crd* ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/${csv}
}

#iterate over all OLM bundles and for each one build a dedicated directory
function packBundles {
    csvs=$1
    for csv in $(ls -1 $csvs); do
        if [[ $csv =~ "csv" ]]; then
            echo "pack bundle for CSV "${csv}
            packBundle $csv
        fi
    done
}

${BUILD_DIR}/build-copy-artifacts.sh "${OLM_CATALOG_INIT_PATH}"

#copy Dockerfile 
cp ${BUILD_DIR}/docker/${CDI_OLM_CATALOG}/* ${OLM_CATALOG_OUT_PATH}/

#Build directory structure expected by olm-catalog-registry
# olm-catalog---
#              |
#              ---cdi--
#                     |
#                     package manifest
#                     ${CSV_VERSION} directory --
#                                               |
#                                               crd manifests
#                                               csv manifests
#   
#
mkdir -p ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}
mkdir -p ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}
cp ${OLM_MANIFESTS_SRC_PATH}/*package* ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/

packBundles ${OLM_MANIFESTS_SRC_PATH}







