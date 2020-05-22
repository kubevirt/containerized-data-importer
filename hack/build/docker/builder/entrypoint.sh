#!/usr/bin/env bash

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

# Keep the pipefail here, or the unit tests won't return an error when needed.
set -eo pipefail

source /etc/profile.d/gimme.sh

export JAVA_HOME=/usr/lib/jvm/jre-1.8.0-openjdk
export PATH=${GOPATH}/bin:/opt/gradle/gradle-4.3.1/bin:$PATH

eval "$@"

if [ "$KUBEVIRTCI_RUNTIME" != "podman" ] && [ -n ${RUN_UID} ] && [ -n ${RUN_GID} ]; then
    find . -user root -exec chown -h ${RUN_UID}:${RUN_GID} {} \;
fi
