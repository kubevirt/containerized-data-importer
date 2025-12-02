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

# Filter test targets based on architecture compatibility
case "${ARCHITECTURE}" in
  x86_64|crossbuild-x86_64)
    TEST_PUSH_TARGETS=(${TEST_PUSH_TARGETS:-$FUNC_TEST_INIT $FUNC_TEST_HTTP $FUNC_TEST_REGISTRY $FUNC_TEST_REGISTRY_POPULATE $FUNC_TEST_REGISTRY_INIT $FUNC_TEST_BAD_WEBSERVER $FUNC_TEST_PROXY $FUNC_TEST_POPULATOR $FUNC_TEST_IMAGEIO $FUNC_TEST_IMAGEIO_INIT $FUNC_TEST_VCENTER_SIMULATOR $FUNC_TEST_TINYCORE $FUNC_TEST_VDDK_INIT $FUNC_TEST_VDDK_TEST $FUNC_TEST_CIRROS_QCOW2})
    ;;
  aarch64|crossbuild-aarch64)
    TEST_PUSH_TARGETS=(${TEST_PUSH_TARGETS:-$FUNC_TEST_INIT $FUNC_TEST_HTTP $FUNC_TEST_REGISTRY $FUNC_TEST_REGISTRY_POPULATE $FUNC_TEST_REGISTRY_INIT $FUNC_TEST_BAD_WEBSERVER $FUNC_TEST_PROXY $FUNC_TEST_POPULATOR $FUNC_TEST_IMAGEIO $FUNC_TEST_IMAGEIO_INIT $FUNC_TEST_TINYCORE $FUNC_TEST_CIRROS_QCOW2})
    ;;
  s390x|crossbuild-s390x)
    TEST_PUSH_TARGETS=(${TEST_PUSH_TARGETS:-$FUNC_TEST_INIT $FUNC_TEST_HTTP $FUNC_TEST_REGISTRY $FUNC_TEST_REGISTRY_POPULATE $FUNC_TEST_REGISTRY_INIT $FUNC_TEST_BAD_WEBSERVER $FUNC_TEST_PROXY $FUNC_TEST_POPULATOR $FUNC_TEST_TINYCORE $FUNC_TEST_CIRROS_QCOW2})
    ;;
esac

PUSH_TARGETS=(${PUSH_TARGETS:-$CONTROLLER_IMAGE_NAME $IMPORTER_IMAGE_NAME $CLONER_IMAGE_NAME $APISERVER_IMAGE_NAME $UPLOADPROXY_IMAGE_NAME $UPLOADSERVER_IMAGE_NAME $OPERATOR_IMAGE_NAME})


echo "docker_prefix: $DOCKER_PREFIX, docker_tag: $DOCKER_TAG"
for target in ${PUSH_TARGETS[@]}; do
    echo "Pushing: $target"
    bazel run \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${DOCKER_PREFIX} \
        --define container_tag=${DOCKER_TAG} \
        --host_force_python=PY3 \
        //:push-${target}
done

# Push test images
for target in ${TEST_PUSH_TARGETS[@]}; do
    echo "Pushing test image: $target"
    bazel run \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${DOCKER_PREFIX} \
        --define container_tag=${DOCKER_TAG} \
        --host_force_python=PY3 \
        //:push-${target}
done

rm -rf ${DIGESTS_DIR}/${ARCHITECTURE}
mkdir -p ${DIGESTS_DIR}/${ARCHITECTURE}

for f in $(find bazel-bin/ -name '*.digest'); do
    dir=${DIGESTS_DIR}/${ARCHITECTURE}/$(dirname $f)
    mkdir -p ${dir}
    cp -f ${f} ${dir}/$(basename ${f})
done
