#!/bin/bash
#
# This file is part of the KubeVirt project
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Copyright 2019 Red Hat, Inc.
#

set -e

source hack/build/common.sh
source hack/build/config.sh

if [ -n "$DOCKER_CA_CERT_FILE" ]; then
    /usr/bin/update-ca-trust
fi

BASE_TEST_IMAGES=(
    "cdi-func-test-bad-webserver"
    "cdi-func-test-proxy"
    "cdi-func-test-sample-populator"
    "cdi-func-test-file-host-init"
    "cdi-func-test-file-host-http"
    "cdi-func-test-registry-init"
    "cdi-func-test-tinycore"
    "cdi-func-test-registry-populate"
    "cdi-func-test-registry"
)

declare -A ARCH_ADDITIONAL_IMAGES
ARCH_ADDITIONAL_IMAGES[x86_64]="imageio-init vddk-init vddk-test vcenter-simulator cdi-func-test-imageio cdi-func-test-cirros-qcow2"
ARCH_ADDITIONAL_IMAGES[aarch64]="imageio-init cdi-func-test-cirros-qcow2"
ARCH_ADDITIONAL_IMAGES[s390x]="vcenter-simulator"

TEST_IMAGES=("${BASE_TEST_IMAGES[@]}")

for img in ${ARCH_ADDITIONAL_IMAGES[$ARCHITECTURE]}; do
    TEST_IMAGES+=("$img")
done

PUSH_TARGETS=(${PUSH_TARGETS:-$CONTROLLER_IMAGE_NAME $IMPORTER_IMAGE_NAME $CLONER_IMAGE_NAME $APISERVER_IMAGE_NAME $UPLOADPROXY_IMAGE_NAME $UPLOADSERVER_IMAGE_NAME $OPERATOR_IMAGE_NAME})
PUSH_TARGETS+=("${TEST_IMAGES[@]}")

echo "docker_prefix: $DOCKER_PREFIX, docker_tag: $DOCKER_TAG"
for target in ${PUSH_TARGETS[@]}; do
    echo "Pushing: $target"
    bazel run \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${DOCKER_PREFIX} \
        --define container_tag=${DOCKER_TAG} \
        --define push_target=${target} \
        --host_force_python=PY3 \
        //:push-${target}
done

rm -rf ${DIGESTS_DIR}/${ARCHITECTURE}
mkdir -p ${DIGESTS_DIR}/${ARCHITECTURE}

for target in ${PUSH_TARGETS[@]}; do
    dir=${DIGESTS_DIR}/${ARCHITECTURE}/${target}
    mkdir -p ${dir}
    touch ${dir}/${target}.image
done
