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

for tag in ${docker_tag}; do
    bazel build \
        --verbose_failures \
        --config=${ARCHITECTURE} \
        --define container_prefix=${docker_prefix} \
        --define container_tag=${tag} \
        --host_force_python=PY3 \
        //:test-container-images //cmd/cdi-operator:cdi-operator-image //cmd/cdi-controller:cdi-controller-image //cmd/cdi-apiserver:cdi-apiserver-image //cmd/cdi-cloner:cdi-cloner-image //cmd/cdi-importer:cdi-importer-image //cmd/cdi-uploadproxy:cdi-uploadproxy-image //cmd/cdi-uploadserver:cdi-uploadserver-image
done
