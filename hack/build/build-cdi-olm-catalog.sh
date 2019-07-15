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

csv_tool="${BIN_DIR}/cdi-olm-catalog"

(cd "${CDI_DIR}/tools/cdi-olm-catalog/" && go build -o "${csv_tool}" ./...)


OUT_PATH="${OUT_DIR}"
OLM_CATALOG_INIT_PATH="tools/${CDI_OLM_CATALOG}"
OLM_CATALOG_OUT_PATH=${OUT_PATH}/${OLM_CATALOG_INIT_PATH}
OLM_MANIFESTS_SRC_PATH=${OUT_PATH}/manifests/release/olm/bundle
OLM_MANIFESTS_DIR=olm-catalog
OLM_PACKAGE=${QUAY_REPOSITORY}
OLM_TMP_CRDS=${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/crds

#create directory for olm catalog container
mkdir -p  "${OLM_CATALOG_OUT_PATH}"


#extract CRD version supported by given CSV
function getCSVCRDVersion {
    csv=$1
    local crdVersion
    crdVersion=$(${csv_tool} --cmd get-csv-crd-version --csv-file $csv --crd-kind "CDI")
    echo $crdVersion
}

#locate CRD file with the provided version
function getCRD {
    crdversion=$1
    crdslocation=$2
    
    local crdfilename
    crdfilename="none"
    for crd in $(ls $crdslocation); do
       if [[ $crd =~ "crd" ]]; then
           if [[ $(${csv_tool} --cmd get-crd-version --crd-file $crdslocation/$crd --crd-kind "CDI") == "$crdVersion" ]]; then
             crdfilename=$crd
             break
          fi
       fi
    done

    if [ "$crdfilename" = "none" ]; then
        echo "Error: No matching CRD for version "$crdversion
        exit -1
    fi
    echo $crdfilename
}

#copy OLM manifests of a provided csv version to a dedicated directory
function packBundle {
    csv=$1
    crds=$2

    mkdir -p ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/${csv}
    cp ${OLM_MANIFESTS_SRC_PATH}/${csv} ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/${csv}

    crdVersion=$(getCSVCRDVersion "${OLM_MANIFESTS_SRC_PATH}/${csv}")
    crdFile=$(getCRD $crdVersion $crds)

    cp $crds/$crdFile ${OLM_CATALOG_OUT_PATH}/${OLM_MANIFESTS_DIR}/${OLM_PACKAGE}/${csv}/cdi-crds.yaml
}

#copy crds unified manifiest to temp location and split it into manifests per crd
function prepareCRDFiles {
    crds=$1
    cur=$(pwd)
    cp ${OLM_MANIFESTS_SRC_PATH}/*crds* $crds
    cd $crds
    csplit --digits=3  --quiet  --elide-empty-files --prefix=cdi-crd --suffix-format=%03d.yaml $crds/cdi-crds.yaml "/--/" "{*}"
    rm -rf $crds/cdi-crds.yaml
    cd $cur

}

#iterate over all OLM bundles and for each one build a dedicated directory
function packBundles {
   
    csvs=$1
    crds=${OLM_TMP_CRDS}
    mkdir -p $crds
    prepareCRDFiles  $crds

    for csv in $(ls -1 $csvs); do
        if [[ $csv =~ "csv" ]]; then
            echo "pack bundle for CSV "${csv}
            packBundle $csv $crds
        fi
    done

    #cleanup temp crds directory
    rm -rf $crds
}

${BUILD_DIR}/build-copy-artifacts.sh "${OLM_CATALOG_INIT_PATH}"

#copy Dockerfile 
cp ${BUILD_DIR}/docker/${CDI_OLM_CATALOG}/* ${OLM_CATALOG_OUT_PATH}/
cp "${OLM_CATALOG_INIT_PATH}"/* ${OLM_CATALOG_OUT_PATH}/

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
