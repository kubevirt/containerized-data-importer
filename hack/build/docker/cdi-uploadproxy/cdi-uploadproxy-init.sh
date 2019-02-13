#!/bin/sh

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

set -e

if [ "$#" -eq 0 ]; then
    echo "Script requires max wait time in seconds"
    exit 1
fi

deployments=("cdi-deployment" "cdi-apiserver")
namespace="$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)"
certfile="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"

timeout=$1
i=0

while [ $i -lt $timeout ]; do
    err=0
    
    for deployment in "${deployments[@]}"; do
        cnt=$(curl -s --cacert $certfile --header "Authorization: Bearer $token" \
            https://kubernetes.default.svc/apis/apps/v1/namespaces/$namespace/deployments/$deployment \
	        | jq -r '.status.readyReplicas // 0')

        if [ $? -ne 0 ] || [ $cnt -eq 0 ]; then
            echo "Deployment $deployment is NOT ready"
            err=1
            break
        fi

        echo "Deployment $deployment is ready"
    done

    if [ $err -eq 0 ]; then
        echo "All deployments ready, exiting"
        exit 0
    fi

    echo "sleeping..."
    sleep 2
    i=$((i + 2))
done

echo "Timed out waiting for deployments to start after $timeout seconds"
exit 1
