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

docker_tag=$DOCKER_TAG

declare -A IMAGE_TARGET_MAP
IMAGE_TARGET_MAP["cdi-func-test-bad-webserver"]="//tools/cdi-func-test-bad-webserver:cdi-func-test-bad-webserver-image"
IMAGE_TARGET_MAP["cdi-func-test-proxy"]="//tools/cdi-func-test-proxy:cdi-func-test-proxy-image"
IMAGE_TARGET_MAP["cdi-func-test-sample-populator"]="//tools/cdi-func-test-sample-populator:cdi-func-test-sample-populator-image"
IMAGE_TARGET_MAP["cdi-func-test-file-host-init"]="//tools/cdi-func-test-file-host-init:cdi-func-test-file-host-init-image"
IMAGE_TARGET_MAP["cdi-func-test-file-host-http"]="//tools/cdi-func-test-file-host-init:cdi-func-test-file-host-http-image"
IMAGE_TARGET_MAP["cdi-func-test-registry-init"]="//tools/cdi-func-test-registry-init:cdi-func-test-registry-init-image"
IMAGE_TARGET_MAP["cdi-func-test-registry-populate"]="//tools/cdi-func-test-registry-init:cdi-func-test-registry-populate-image"
IMAGE_TARGET_MAP["cdi-func-test-registry"]="//tools/cdi-func-test-registry-init:cdi-func-test-registry-image"
IMAGE_TARGET_MAP["cdi-func-test-tinycore"]="//tests:cdi-func-test-tinycore"
IMAGE_TARGET_MAP["cdi-func-test-cirros-qcow2"]="//tests:cdi-func-test-cirros-qcow2"
IMAGE_TARGET_MAP["imageio-init"]="//tools/imageio-init:imageio-init-image"
IMAGE_TARGET_MAP["cdi-func-test-imageio"]="//tools/image-io:cdi-func-test-imageio-image"
IMAGE_TARGET_MAP["vddk-init"]="//tools/vddk-init:vddk-init-image"
IMAGE_TARGET_MAP["vddk-test"]="//tools/vddk-test:vddk-test-image"
IMAGE_TARGET_MAP["vcenter-simulator"]="//tools/vddk-test:vcenter-simulator-image"

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

BAZEL_TARGETS=(
    "//cmd/cdi-operator:cdi-operator-image"
    "//cmd/cdi-controller:cdi-controller-image"
    "//cmd/cdi-apiserver:cdi-apiserver-image"
    "//cmd/cdi-cloner:cdi-cloner-image"
    "//cmd/cdi-importer:cdi-importer-image"
    "//cmd/cdi-uploadproxy:cdi-uploadproxy-image"
    "//cmd/cdi-uploadserver:cdi-uploadserver-image"
)

for img in "${BASE_TEST_IMAGES[@]}"; do
    BAZEL_TARGETS+=("${IMAGE_TARGET_MAP[$img]}")
done

for img in ${ARCH_ADDITIONAL_IMAGES[$ARCHITECTURE]}; do
    BAZEL_TARGETS+=("${IMAGE_TARGET_MAP[$img]}")
done

for tag in ${docker_tag}; do
    bazel build \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${docker_prefix} \
        --define container_tag=${tag} \
        --host_force_python=PY3 \
        "${BAZEL_TARGETS[@]}"
done

rm -rf ${DIGESTS_DIR}/${ARCHITECTURE}
mkdir -p ${DIGESTS_DIR}/${ARCHITECTURE}

for target in ${PUSH_TARGETS[@]}; do
    dir=${DIGESTS_DIR}/${ARCHITECTURE}/${target}
    mkdir -p ${dir}
    touch ${dir}/${target}.image
done
