#!/bin/sh
#
# Copyright 2022 The CDI Authors.
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
# Common functions used by test scripts
#

get_latest_release() {
    curl -s "https://api.github.com/repos/$1/releases/latest" | # Get latest release from GitHub api
        grep '"tag_name":' | # Get tag line
        sed -E 's/.*"([^"]+)".*/\1/' # Pluck JSON value (avoid jq)
}

get_previous_y_release() {
    curl -s "https://api.github.com/repos/$1/releases" |
        grep '"tag_name":' |
        sed -E 's/.*"([^"]+)".*/\1/' |
        sort -V | grep -v rc | grep "$2" -B 100 | grep -v "$2" | tail -n 1 | xargs
}

# Follow KubeVirt's CentOS Stream selection for kubevirtci providers.
# k8s-1.37 and later select CS10; older and other providers default to CS9.
# An explicit CDI_CENTOS_STREAM_VERSION takes precedence.
detect_centos_stream_version() {
    if [[ -n "${CDI_CENTOS_STREAM_VERSION:-}" ]]; then
        return
    fi
    local provider_version
    provider_version=$(echo "${KUBEVIRT_PROVIDER:-}" | sed -n 's/^k8s-\([0-9]*\.[0-9]*\).*/\1/p')
    if [[ -n "${provider_version}" ]] &&
        [[ "$(printf '%s\n' 1.37 "${provider_version}" | sort -V | head -n1)" == "1.37" ]]; then
        export CDI_CENTOS_STREAM_VERSION=10
    else
        export CDI_CENTOS_STREAM_VERSION=9
    fi
}
