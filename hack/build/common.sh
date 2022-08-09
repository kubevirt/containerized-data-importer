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

determine_cri_bin() {
    if [ "${KUBEVIRTCI_RUNTIME-}" = "podman" ]; then
        echo "podman --remote --url=unix://${XDG_RUNTIME_DIR-}/podman/podman.sock"
    elif [ "${KUBEVIRTCI_RUNTIME-}" = "docker" ]; then
        echo docker
    else
        if curl --unix-socket "${XDG_RUNTIME_DIR-}/podman/podman.sock" http://d/v3.0.0/libpod/info >/dev/null 2>&1; then
            echo "podman --remote --url=unix://${XDG_RUNTIME_DIR-}/podman/podman.sock"
        elif docker ps >/dev/null 2>&1; then
            echo docker
        else
            echo ""
        fi
    fi
}

SCRIPT_DIR=$(dirname "$(readlink -f "${BASH_SOURCE[0]}")")
CDI_DIR=$(cd "$SCRIPT_DIR/../../" && pwd -P)
CDI_GO_PACKAGE=kubevirt.io/containerized-data-importer
BIN_DIR=${CDI_DIR}/bin
OUT_DIR=${CDI_DIR}/_out
CMD_OUT_DIR=${OUT_DIR}/cmd
TESTS_OUT_DIR=${OUT_DIR}/tests
BUILD_DIR=${CDI_DIR}/hack/build
MANIFEST_TEMPLATE_DIR=${CDI_DIR}/manifests/templates
MANIFEST_GENERATED_DIR=${CDI_DIR}/manifests/generated
SOURCE_DIRS="pkg tests tools"
APIDOCS_OUT_DIR=${OUT_DIR}/apidocs
CACHE_DIR=${OUT_DIR}/gocache
VENDOR_DIR=${CDI_DIR}/vendor
ARCHITECTURE="${BUILD_ARCH:-$(uname -m)}"
HOST_ARCHITECTURE="$(uname -m)"
CDI_CRI="$(determine_cri_bin)"
if [ "${CDI_CRI}" = "docker" ]; then
   CDI_CONTAINER_BUILDCMD="docker"
else
   CDI_CONTAINER_BUILDCMD="buildah"
fi
