#!/bin/bash

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

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

#all generated files are placed in manifests/generated
function generateResourceManifest() {
    generator=$1
    targetDir=$2
    resourceType=$3
    resourceGroup=$4
    filename=$5

    manifestName=$filename
    manifestNamej2=$filename".j2"

    rm -rf ${targetDir}/$manifestName
    rm -rf ${targetDir}/$manifestNamej2
    (
        ${generator} -resource-type=${resourceType} \
            -resource-group=${resourceGroup} \
            -docker-repo="${DOCKER_PREFIX}" \
            -docker-tag="${DOCKER_TAG}" \
            -operator-version="${DOCKER_TAG}" \
            -deploy-cluster-resources="true" \
            -operator-image="${DOCKER_PREFIX}/${OPERATOR_IMAGE_NAME}:${DOCKER_TAG}" \
            -controller-image="${DOCKER_PREFIX}/${CONTROLLER_IMAGE_NAME}:${DOCKER_TAG}" \
            -importer-image="${DOCKER_PREFIX}/${IMPORTER_IMAGE_NAME}:${DOCKER_TAG}" \
            -cloner-image="${DOCKER_PREFIX}/${CLONER_IMAGE_NAME}:${DOCKER_TAG}" \
            -apiserver-image="${DOCKER_PREFIX}/${APISERVER_IMAGE_NAME}:${DOCKER_TAG}" \
            -uploadproxy-image="${DOCKER_PREFIX}/${UPLOADPROXY_IMAGE_NAME}:${DOCKER_TAG}" \
            -uploadserver-image="${DOCKER_PREFIX}/${UPLOADSERVER_IMAGE_NAME}:${DOCKER_TAG}" \
            -verbosity="${VERBOSITY}" \
            -pull-policy="${PULL_POLICY}" \
            -namespace="${NAMESPACE}"
    ) 1>>"${targetDir}/"$manifestName

    (
        ${generator} -resource-type=${resourceType} \
            -resource-group=${resourceGroup} \
            -docker-repo="{{ docker_prefix }}" \
            -docker-tag="{{ docker_tag }}" \
            -operator-version="{{ operator_version }}" \
            -deploy-cluster-resources="true" \
            -operator-image="{{ operator_image_name }}" \
            -controller-image="{{ controller_image }}" \
            -importer-image="{{ importer_image }}" \
            -cloner-image="{{ cloner_image }}" \
            -apiserver-image="{{ apiserver_image }}" \
            -uploadproxy-image="{{ uploadproxy_image }}" \
            -uploadserver-image="{{ uploadserver_image }}" \
            -verbosity="${VERBOSITY}" \
            -pull-policy="{{ pull_policy }}" \
            -namespace="{{ cdi_namespace }}"
    ) 1>>"${targetDir}/"$manifestNamej2

    # Remove empty lines at the end of files which are added by go templating
    find ${targetDir}/ -type f -exec sed -i {} -e '${/^$/d;}' \;
}

function processDirTemplates() {
    inTmplPath=$1           #Path to directory from which to take manifests templates for processing
    outFinalManifestPath=$2 #Path to which to store final manifests version
    outTmplPath=$3          #Path to which to store templated manifests version
    generator=$4            #generator binary
    genManifestsDir=$5      #path where manifests generated from code are stored

    rm -rf $outFinalManifestPath
    rm -rf $outTmplPath
    mkdir -p $outFinalManifestPath
    mkdir -p $outTmplPath

    templates="$(find "${inTmplPath}" -maxdepth 1 -name "*.in" -type f)"
    for tmpl in ${templates}; do
        tmpl_dir="$(cd "$(dirname "${tmpl}")" && pwd -P)"
        tmpl_filename="$(basename ${tmpl})"
        tmpl="${tmpl_dir}/${tmpl_filename}"
        populateResourceManifest $generator $outFinalManifestPath $outTmplPath $tmpl $genManifestsDir $outFinalManifestPath
    done
}

# all templated final manifsets are located in _out/manifests/
# all templated  manifsets are located in _out/manifests/templates
function populateResourceManifest() {
    generator=$1
    targetDir=$2
    tmplTargetDir=$3
    tmpl=$4
    generatedManifests=$5
    outDir=$6

    bundleOut="none"
    tmplBundleOut="none"
    outfile=$(basename -s .in "${tmpl}")

    if [[ $tmpl == *"VERSION"* ]]; then
        #if the processed template is CSV - pass output directory for olm bundle
        outfile=${outfile/VERSION/${CSV_VERSION}}
        bundleOut="${outDir}"
        tmplBundleOut="${tmplTargetDir}"
    fi
    (
        ${generator} -template="${tmpl}" \
            -docker-repo="${DOCKER_PREFIX}" \
            -docker-tag="${DOCKER_TAG}" \
            -operator-version="${DOCKER_TAG}" \
            -deploy-cluster-resources="true" \
            -operator-image="${DOCKER_PREFIX}/${OPERATOR_IMAGE_NAME}:${DOCKER_TAG}" \
            -controller-image="${DOCKER_PREFIX}/${CONTROLLER_IMAGE_NAME}:${DOCKER_TAG}" \
            -importer-image="${DOCKER_PREFIX}/${IMPORTER_IMAGE_NAME}:${DOCKER_TAG}" \
            -cloner-image="${DOCKER_PREFIX}/${CLONER_IMAGE_NAME}:${DOCKER_TAG}" \
            -apiserver-image="${DOCKER_PREFIX}/${APISERVER_IMAGE_NAME}:${DOCKER_TAG}" \
            -uploadproxy-image="${DOCKER_PREFIX}/${UPLOADPROXY_IMAGE_NAME}:${DOCKER_TAG}" \
            -uploadserver-image="${DOCKER_PREFIX}/${UPLOADSERVER_IMAGE_NAME}:${DOCKER_TAG}" \
            -verbosity="${VERBOSITY}" \
            -pull-policy="${PULL_POLICY}" \
            -cr-name="${CR_NAME}" \
            -namespace="${NAMESPACE}" \
            -generated-manifests-path=${generatedManifests}
    ) 1>>"${targetDir}/"$outfile

    (
        ${generator} -template="${tmpl}" \
            -docker-repo="{{ docker_prefix }}" \
            -docker-tag="{{ docker_tag }}" \
            -operator-version="{{ operator_version }}" \
            -deploy-cluster-resources="true" \
            -operator-image="{{ operator_image_name }}" \
            -controller-image="{{ controller_image }}" \
            -importer-image="{{ importer_image }}" \
            -cloner-image="{{ cloner_image }}" \
            -apiserver-image="{{ apiserver_image }}" \
            -uploadproxy-image="{{ uploadproxy_image }}" \
            -uploadserver-image="{{ uploadserver_image }}" \
            -verbosity="${VERBOSITY}" \
            -pull-policy="{{ pull_policy }}" \
            -namespace="{{ cdi_namespace }}" \
            -generated-manifests-path=${generatedManifests}
    ) 1>>"${tmplTargetDir}/"$outfile".j2"

    # Remove empty lines at the end of files which are added by go templating
    find ${targetDir}/ -type f -exec sed -i {} -e '${/^$/d;}' \;
    find ${tmplTargetDir}/ -type f -exec sed -i {} -e '${/^$/d;}' \;
}
