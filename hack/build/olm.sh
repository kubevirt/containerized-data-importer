#!/usr/bin/env bash
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

source $(dirname "$0")/common.sh
source $(dirname "$0")/config.sh

if [ $# -ne 1 ]; then
    echo "usage: ${0} verify | push"
fi

BUNDLE_DIR=${OUT_DIR}/manifests/release/olm/bundle
echo "using these manifests:"
ls ${BUNDLE_DIR}

case ${1} in

verify)

    IFS=
    result=$(operator-courier verify ${BUNDLE_DIR} 2>&1)
    echo $result

    if [[ $result =~ "ERROR" ]]; then
        echo "olm verify failed!"
        exit 1
    fi

    echo "olm verify success!"
    exit 0

    ;;

push)

    if  [[ $CSV_VERSION == "0.0.0" ]]; then
        echo "ERROR: must not push to quay CSV_VERSION="$CSV_VERSION
        exit 1
    fi

    if [[ -z "$QUAY_USERNAME" ]] || [[ -z "$QUAY_USERNAME" ]]; then
        echo "please set QUAY_USERNAME, QUAY_PASSWORD"
        exit 1
    fi

    echo "getting auth token from Quay"
    AUTH_TOKEN=$(curl -sH "Content-Type: application/json" -XPOST https://quay.io/cnr/api/v1/users/login -d '
        {
            "user": {
                "username": "'"${QUAY_USERNAME}"'",
                "password": "'"${QUAY_PASSWORD}"'"
            }
        }' | jq -r '.token')

    echo "pushing bundle"
    
    operator-courier push ${BUNDLE_DIR} ${QUAY_NAMESPACE} ${QUAY_REPOSITORY} ${CSV_VERSION} "$AUTH_TOKEN"

    ;;
esac

