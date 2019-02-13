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

if [ "$#" -eq 0 ]; then
    echo "Script requires max wait time in seconds"
    exit 1
fi

service="cdi-api"
namespace="$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)"
certfile="/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
token="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"

timeout=$1
i=0

while [ $i -lt $timeout ]; do
    cnt=$(curl -s --cacert $certfile --header "Authorization: Bearer $token" \
        https://kubernetes.default.svc/api/v1/namespaces/$namespace/endpoints/$service \
	    | jq -r '.subsets[].addresses | length')
    
    if [ $? -eq 0 ] && [ $cnt -gt 0 ]; then
        echo "$service ready, exiting"
        exit 0
    fi

    sleep 2
    i=$((i + 2)) 
done

echo "Service $service not running after $timeout secs"
exit 1
