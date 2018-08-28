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

CONTROLLER="cdi-controller"
IMPORTER="cdi-importer"
CLONER="cdi-cloner"
FUNC_TEST_INIT="cdi-func-test-file-host-init"
FUNC_TEST_HTTP="cdi-func-test-file-host-http"

BINARIES="cmd/${CONTROLLER} cmd/${IMPORTER} tools/${FUNC_TEST_INIT}"
CDI_PKGS="cmd/ pkg/ test/"

CONTROLLER_MAIN="cmd/${CONTROLLER}"
IMPORTER_MAIN="cmd/${IMPORTER}"
CLONER_MAIN="cmd/${CLONER}"

DOCKER_IMAGES="cmd/${CONTROLLER} cmd/${IMPORTER} cmd/${CLONER} tools/${FUNC_TEST_INIT} tools/${FUNC_TEST_HTTP}"
DOCKER_REPO=${DOCKER_REPO:-kubevirt}
DOCKER_TAG=${DOCKER_TAG:-latest}
VERBOSITY=${VERBOSITY:-1}
PULL_POLICY=${PULL_POLICY:-IfNotPresent}
NAMESPACE=${NAMESPACE:-kube-system}

KUBERNETES_IMAGE="k8s-1.10.4@sha256:ee6846957b58e1f56b240d9ba6410f082e4787a4c4f1e0d60f6b907b76146b3e"
OPENSHIFT_IMAGE="os-3.10.0@sha256:cdc9f998e19915b28b5c5be1ccc4c6fa2c8336435f38a37855f75b206977cbc2"

KUBEVIRT_PROVIDER=${KUBEVIRT_PROVIDER:-k8s-1.10.4}

function allPkgs() {
    ret=$(sed "s,kubevirt.io/containerized-data-importer,${CDI_DIR},g" <(go list ./... | grep -v "pkg/client" | sort -u))
    echo "$ret"
}

function parseTestOpts() {
    pkgs=""
    test_args=""
    while [[ $# -gt 0 ]] && [[ $1 != "" ]]; do
        case "${1}" in
        --test-args=*)
            test_args="${1#*=}"
            shift 1
            ;;
        ./*...)
            pkgs="${pkgs} ${1}"
            shift 1
            ;;
        *)
            echo "ABORT: Unrecognized option \"$1\""
            exit 1
            ;;
        esac
    done
}
