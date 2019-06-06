#!/bin/bash -e

source ./cluster/gocli.sh
source ./hack/build/config.sh
source ./cluster/$KUBEVIRT_PROVIDER/provider.sh

echo "Image:${image}"
echo "Starting cluster"
up

verify
