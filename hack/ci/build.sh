#!/usr/bin/env bash

set -euo pipefail

# export DOCKER_PREFIX='kubevirtnightlybuilds'
export DOCKER_TAG="latest"
export KUBEVIRT_PROVIDER=external

# bash -x ./hack/build/build-manifests.sh

bash -x ./hack/ci/build-functest.sh

rm -rf _ci-configs/

# to avoid any permission problems we reset access rights recursively
chmod -R 777 .
