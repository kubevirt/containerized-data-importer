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

if [ -n "$DOCKER_CA_CERT_FILE" ] ; then
    /usr/bin/update-ca-trust
fi 

PUSH_TARGETS=(${PUSH_TARGETS:-$CONTROLLER_IMAGE_NAME $IMPORTER_IMAGE_NAME $CLONER_IMAGE_NAME $APISERVER_IMAGE_NAME $UPLOADPROXY_IMAGE_NAME $UPLOADSERVER_IMAGE_NAME $OPERATOR_IMAGE_NAME})

echo "docker_prefix: $DOCKER_PREFIX, docker_tag: $DOCKER_TAG"
for target in ${PUSH_TARGETS[@]}; do
    echo "Pushing: $target"
    bazel run \
        --verbose_failures \
        --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64_cgo \
        --define container_prefix=${DOCKER_PREFIX} \
        --define container_tag=${DOCKER_TAG} \
        --host_force_python=PY3 \
        //:push-${target}
done

bazel run \
    --verbose_failures \
    --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64_cgo \
    --define container_prefix=${DOCKER_PREFIX} \
    --define container_tag=${DOCKER_TAG} \
    --host_force_python=PY3 \
    //:push-test-images
