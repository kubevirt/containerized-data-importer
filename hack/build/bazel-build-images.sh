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

COMMON_IMAGE_TARGETS=(
    //cmd/cdi-operator:cdi-operator-image
    //cmd/cdi-controller:cdi-controller-image
    //cmd/cdi-apiserver:cdi-apiserver-image
    //cmd/cdi-cloner:cdi-cloner-image
    //cmd/cdi-importer:cdi-importer-image
    //cmd/cdi-uploadproxy:cdi-uploadproxy-image
    //cmd/cdi-uploadserver:cdi-uploadserver-image
)

TEST_IMAGE_TARGETS=(
    //tools/cdi-func-test-bad-webserver:cdi-func-test-bad-webserver-image
    //tools/cdi-func-test-proxy:cdi-func-test-proxy-image
    //tools/cdi-func-test-sample-populator:cdi-func-test-sample-populator-image
    //tools/cdi-func-test-file-host-init:cdi-func-test-file-host-init-image
    //tools/cdi-func-test-file-host-init:cdi-func-test-file-host-http-image
    //tools/cdi-func-test-registry-init:cdi-func-test-registry-init-image
    //tools/cdi-func-test-registry-init:cdi-func-test-registry-populate-image
    //tools/cdi-func-test-registry-init:cdi-func-test-registry-image
    //tests:cdi-func-test-tinycore
    //tests:cdi-func-test-cirros-qcow2
)

case "${ARCHITECTURE}" in
  x86_64|crossbuild-x86_64)
    TEST_IMAGE_TARGETS+=(
        //tools/imageio-init:imageio-init-image
        //tools/vddk-test:vcenter-simulator
        //tools/vddk-init:vddk-init-image
        //tools/vddk-test:vddk-test-image
        //tools/image-io:cdi-func-test-imageio-image
    )
    ;;
  aarch64|crossbuild-aarch64)
    TEST_IMAGE_TARGETS+=(
        //tools/imageio-init:imageio-init-image
        //tools/image-io:cdi-func-test-imageio-image
    )
    ;;
  s390x|crossbuild-s390x)
    # No additional for now
    ;;
esac

for tag in ${docker_tag}; do
    bazel build \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${docker_prefix} \
        --define container_tag=${tag} \
        --host_force_python=PY3 \
        "${TEST_IMAGE_TARGETS[@]}" \
        "${COMMON_IMAGE_TARGETS[@]}"
done

rm -rf ${DIGESTS_DIR}/${ARCHITECTURE}
mkdir -p ${DIGESTS_DIR}/${ARCHITECTURE}

for f in $(find bazel-bin/ -name '*.digest'); do
    dir=${DIGESTS_DIR}/${ARCHITECTURE}/$(dirname $f)
    mkdir -p ${dir}
    cp -f ${f} ${dir}/$(basename ${f})
done
