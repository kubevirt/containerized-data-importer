#!/usr/bin/env bash

#Copyright 2019 The CDI Authors.
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

set -euo pipefail

source hack/build/config.sh
source hack/build/common.sh

# parsetTestOpts sets 'pkgs' and test_args
parseTestOpts "${@}"
export GO111MODULE=off
test_command="env OPERATOR_DIR=${CDI_DIR} go test -v -coverprofile=.coverprofile -test.timeout 180m ${pkgs} ${test_args:+-args $test_args}"
echo "${test_command}"
${test_command}
