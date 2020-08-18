#!/usr/bin/env bash

#Copyright 2020 The CDI Authors.
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

set -e
script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

BUILDER_SPEC="hack/ci/Dockerfile.ci"

# When building and pushing a new image we do not provide the sha hash
# because docker assigns that for us.
UNTAGGED_BUILDER_IMAGE=kubevirt/cdi-osci-builder

# Build the encapsulated compile and test container
docker build -f ${BUILDER_SPEC} --tag ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG} .

DIGEST=$(docker images --digests | grep ${UNTAGGED_BUILDER_IMAGE} | grep ${BUILDER_TAG} | awk '{ print $4 }')
echo "Image: ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}"
echo "Digest: ${DIGEST}"

docker push ${UNTAGGED_BUILDER_IMAGE}:${BUILDER_TAG}
