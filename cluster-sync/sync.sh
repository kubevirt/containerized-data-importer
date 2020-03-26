#!/bin/bash -e

cdi=$1
cdi="${cdi##*/}"

echo $cdi

source ./hack/build/config.sh
source ./hack/build/common.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/provider.sh

CDI_INSTALL="install-operator"
CDI_NAMESPACE=${CDI_NAMESPACE:-cdi}
CDI_INSTALL_TIMEOUT=${CDI_INSTALL_TIMEOUT:-120}
CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT:-480}

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
DOCKER_PREFIX=$MANIFEST_REGISTRY PULL_POLICY=$(getTestPullPolicy) make manifests
DOCKER_PREFIX=$DOCKER_PREFIX make push

function kill_running_operator {
  out=$(_kubectl get pods -n $CDI_NAMESPACE)
  out=($out)
  length=${#out[@]}
  for ((idx=0; idx<${#out[@]}; idx=idx+5)); do
    if [[ ${out[idx]} == cdi-operator-* ]] && [[ ${out[idx+2]} == "Running" ]]; then
      _kubectl delete pod ${out[idx]} -n $CDI_NAMESPACE --grace-period=0 --force
      return
    fi
  done
}

seed_images

# Install CDI
install_cdi

#wait cdi crd is installed with timeout
wait_cdi_crd_installed $CDI_INSTALL_TIMEOUT

_kubectl apply -f "./_out/manifests/release/cdi-cr.yaml"
echo "Waiting $CDI_AVAILABLE_TIMEOUT seconds for CDI to become available"
_kubectl wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=${CDI_AVAILABLE_TIMEOUT}s

# If we are upgrading, verify our current value.
if [[ ! -z "$UPGRADE_FROM" ]]; then
  UPGRADE_FROM_LIST=( $UPGRADE_FROM )
  for VERSION in ${UPGRADE_FROM_LIST[@]}; do
    echo $VERSION
    if [ "$VERSION" != "${UPGRADE_FROM_LIST[0]}" ]; then
      curl -L "https://github.com/kubevirt/containerized-data-importer/releases/download/${VERSION}/cdi-operator.yaml" --output cdi-operator.yaml
      sed -i "0,/name: cdi/{s/name: cdi/name: $CDI_NAMESPACE/}" cdi-operator.yaml
      sed -i "s/namespace: cdi/namespace: $CDI_NAMESPACE/g" cdi-operator.yaml
      echo $(cat cdi-operator.yaml)
      _kubectl apply -f cdi-operator.yaml
    fi
    retry_counter=0
    kill_count=0
    while [[ $retry_counter -lt 10 ]] && [ "$operator_version" != "$VERSION" ]; do
      cdi_cr_phase=`_kubectl get CDI -o=jsonpath='{.items[*].status.phase}{"\n"}'`
      observed_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.observedVersion}{"\n"}'`
      target_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.targetVersion}{"\n"}'`
      operator_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.operatorVersion}{"\n"}'`
      echo "Phase: $cdi_cr_phase, observedVersion: $observed_version, operatorVersion: $operator_version, targetVersion: $target_version"
      retry_counter=$((retry_counter + 1))
      if [[ $kill_count -lt 1 ]]; then
        kill_running_operator
        kill_count=$((kill_count + 1))
      fi
      _kubectl get pods -n $CDI_NAMESPACE
      sleep 5
    done
    if [ $retry_counter -eq 10 ]; then
    echo "Unable to deploy to version $VERSION"
    cdi_obj=$(_kubectl get CDI -o yaml)
    echo $cdi_obj
    exit 1
    fi
    echo "Currently at version: $VERSION"
  done
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
    _kubectl get pods -n $CDI_NAMESPACE
  sleep 1
  done
  if [ $retry_counter -eq 60 ]; then
	echo "Unable to deploy to latest version"
	cdi_obj=$(_kubectl get CDI -o yaml)
	echo $cdi_obj
	exit 1
  fi
  echo "Waiting $CDI_AVAILABLE_TIMEOUT seconds for CDI to become available"
  _kubectl wait cdis.cdi.kubevirt.io/cdi --for=condition=Available --timeout=${CDI_AVAILABLE_TIMEOUT}s
fi

configure_storage

# Start functional test HTTP server.
# We skip the functional test additions for external provider for now, as they're specific
if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
  _kubectl apply -f "./_out/manifests/file-host.yaml"
  _kubectl apply -f "./_out/manifests/registry-host.yaml"
  # Imageio test service:
  _kubectl apply -f "./_out/manifests/imageio.yaml"
fi
