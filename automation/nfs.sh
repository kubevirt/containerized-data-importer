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
# Copyright 2022 Red Hat, Inc.
#

set -ex
export TARGET=k8s-1.32
export KUBEVIRT_DEPLOY_NFS_CSI=true
export KUBEVIRT_STORAGE=nfs
export KUBEVIRT_NFS_DIR=${KUBEVIRT_NFS_DIR:-/var/lib/containers/nfs-data}
export CDI_E2E_SKIP=Destructive
automation/test.sh
