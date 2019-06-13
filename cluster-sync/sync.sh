#!/bin/bash -e

cdi=$1
cdi="${cdi##*/}"

echo $cdi

source ./hack/build/config.sh
source ./hack/build/common.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/provider.sh

CDI_NAMESPACE=${CDI_NAMESPACE:-cdi}

# Set controller verbosity to 3 for functional tests.
export VERBOSITY=3

PULL_POLICY=$(getTestPullPolicy)
# The default DOCKER_PREFIX is set to kubevirt and used for builds, however we don't use that for cluster-sync
# instead we use a local registry; so here we'll check for anything != "external"
# wel also confuse this by swapping the setting of the DOCKER_PREFIX variable around based on it's context, for
# build and push it's localhost, but for manifests, we sneak in a change to point a registry container on the
# kubernetes cluster.  So, we introduced this MANIFEST_REGISTRY variable specifically to deal with that and not
# have to refactor/rewrite any of the code that works currently.
MANIFEST_REGISTRY=$DOCKER_PREFIX
if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
  registry=${IMAGE_REGISTRY:-localhost:$(_port registry)}
  DOCKER_PREFIX=${registry}
  MANIFEST_REGISTRY="registry:5000"
fi

# Need to set the DOCKER_PREFIX appropriately in the call to `make docker push`, otherwise make will just pass in the default `kubevirt`
DOCKER_PREFIX=$DOCKER_PREFIX make docker push
DOCKER_PREFIX=$MANIFEST_REGISTRY PULL_POLICY=$(getTestPullPolicy) make manifests

seed_images

configure_local_storage

# Install CDI
_kubectl apply -f "./_out/manifests/release/cdi-operator.yaml" 
_kubectl apply -f "./_out/manifests/release/cdi-cr.yaml"
_kubectl wait cdis.cdi.kubevirt.io/cdi --for=condition=running --timeout=120s

# Start functional test HTTP server.
# We skip the functional test additions for external provider for now, as they're specific
if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
_kubectl apply -f "./_out/manifests/file-host.yaml"
_kubectl apply -f "./_out/manifests/registry-host.yaml"
_kubectl apply -f "./_out/manifests/block-device.yaml"
fi
