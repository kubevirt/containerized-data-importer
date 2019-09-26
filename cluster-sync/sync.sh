#!/bin/bash -e

cdi=$1
cdi="${cdi##*/}"

echo $cdi

source ./hack/build/config.sh
source ./hack/build/common.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/provider.sh

CDI_INSTALL_OPERATOR="install-operator"
CDI_INSTALL_OLM="install-olm"
CDI_INSTALL=${CDI_INSTALL:-${CDI_INSTALL_OPERATOR}}
CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}
CDI_NAMESPACE=${CDI_NAMESPACE:-cdi}
CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}

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
  QUAY_NAMESPACE="none"
fi

# Need to set the DOCKER_PREFIX appropriately in the call to `make docker push`, otherwise make will just pass in the default `kubevirt`
QUAY_NAMESPACE=$QUAY_NAMESPACE DOCKER_PREFIX=$MANIFEST_REGISTRY PULL_POLICY=$(getTestPullPolicy) make manifests
DOCKER_PREFIX=$DOCKER_PREFIX make docker push

seed_images

configure_local_storage

# Install CDI
install_cdi

#wait cdi crd is installed with timeout
wait_cdi_crd_installed $CDI_INSTALL_TIMEOUT

_kubectl apply -f "./_out/manifests/release/cdi-cr.yaml"
echo "Waiting 480 seconds for CDI to become available"
_kubectl wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=480s

# If we are upgrading, verify our current value.
if [ ! -z $UPGRADE_FROM ]; then
  retry_counter=0
  while [[ $retry_counter -lt 10 ]] && [ -z $cdi_cr_phase ] && [ "$observed_version" != "$UPGRADE_FROM" ]; do
    cdi_cr_phase=`_kubectl get CDI -o=jsonpath='{.items[*].status.phase}{"\n"}'`
    observed_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.observedVersion}{"\n"}'`
    target_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.targetVersion}{"\n"}'`
    operator_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.operatorVersion}{"\n"}'`
    echo "Phase: $cdi_cr_phase, observedVersion: $observed_version, operatorVersion: $operator_version, targetVersion: $target_version"
    retry_counter=$((retry_counter + 1))
  sleep 5
  done
  if [ $retry_counter -eq 10 ]; then
	echo "Unable to deploy to version $UPGRADE_FROM"
	cdi_obj=$(_kubectl get CDI -o yaml)
	echo $cdi_obj
	exit 1
  fi
  echo "Currently at version: $UPGRADE_FROM"
  echo "Upgrading to latest"
  retry_counter=0
  _kubectl apply -f "./_out/manifests/release/cdi-operator.yaml"
  while [[ $retry_counter -lt 60 ]] && [ "$observed_version" != "latest" ]; do
    cdi_cr_phase=`_kubectl get CDI -o=jsonpath='{.items[*].status.phase}{"\n"}'`
    observed_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.observedVersion}{"\n"}'`
    target_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.targetVersion}{"\n"}'`
    operator_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.operatorVersion}{"\n"}'`
    echo "Phase: $cdi_cr_phase, observedVersion: $observed_version, operatorVersion: $operator_version, targetVersion: $target_version"
    retry_counter=$((retry_counter + 1))
  sleep 1
  done
  if [ $retry_counter -eq 60 ]; then
	echo "Unable to deploy to latest version"
	cdi_obj=$(_kubectl get CDI -o yaml)
	echo $cdi_obj
	exit 1
  fi
fi

# Start functional test HTTP server.
# We skip the functional test additions for external provider for now, as they're specific
if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
_kubectl apply -f "./_out/manifests/file-host.yaml"
_kubectl apply -f "./_out/manifests/registry-host.yaml"
fi
