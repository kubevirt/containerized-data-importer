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
CDI_PODS_UPDATE_TIMEOUT=${CDI_PODS_UPDATE_TIMEOUT:-480}
CDI_UPGRADE_RETRY_COUNT=${CDI_UPGRADE_RETRY_COUNT:-60}

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

function check_structural_schema {
  for crd in "$@"; do
    status=$(_kubectl get crd $crd -o jsonpath={.status.conditions[?\(@.type==\"NonStructuralSchema\"\)].status})
    if [ "$status" == "True" ]; then
      echo "ERROR CRD $crd is not a structural schema!, please fix"
      _kubectl get crd $crd -o yaml
      exit 1
    fi
    echo "CRD $crd is a StructuralSchema"
  done
}

function wait_cdi_available {
  echo "Waiting $CDI_AVAILABLE_TIMEOUT seconds for CDI to become available"
  if [ "$KUBEVIRT_PROVIDER" == "os-3.11.0-crio" ]; then
    echo "Openshift 3.11 provider"
    available=$(_kubectl get cdi cdi -o jsonpath={.status.conditions[0].status})
    wait_time=0
    while [[ $available != "True" ]] && [[ $wait_time -lt ${CDI_AVAILABLE_TIMEOUT} ]]; do
      wait_time=$((wait_time + 5))
      sleep 5
      available=$(_kubectl get cdi cdi -o jsonpath={.status.conditions[0].status})
      fix_failed_sdn_pods
    done
  else
    _kubectl wait cdis.cdi.kubevirt.io/${CR_NAME} --for=condition=Available --timeout=${CDI_AVAILABLE_TIMEOUT}s
  fi
}

OLD_CDI_VER_PODS="./_out/tests/old_cdi_ver_pods"
NEW_CDI_VER_PODS="./_out/tests/new_cdi_ver_pods"

# Wait for update of all cdi pods (names, uids and image versions)
# Note it will fail update to the same version
function wait_cdi_pods_updated {
  echo "Waiting $CDI_PODS_UPDATE_TIMEOUT seconds for all CDI pods to update"
  if [ -f $NEW_CDI_VER_PODS ] ; then
    mv $NEW_CDI_VER_PODS $OLD_CDI_VER_PODS
  fi
  wait_time=0
  ret=0
  while [[ $ret -eq 0 ]] && [[ $wait_time -lt ${CDI_PODS_UPDATE_TIMEOUT} ]]; do
    wait_time=$((wait_time + 5))
    _kubectl get pods -n $CDI_NAMESPACE -o=jsonpath='{range .items[*]}{.metadata.name}{"\n"}{.metadata.uid}{"\n"}{.spec.containers[*].image}{"\n"}{end}' > $NEW_CDI_VER_PODS
    if [ -f $OLD_CDI_VER_PODS ] ; then
      grep -qFxf $OLD_CDI_VER_PODS $NEW_CDI_VER_PODS || ret=$?
      if [ $ret -eq 0 ] ; then
        sleep 5
      fi
    else
      ret=1
    fi
  done
  echo "Waited $wait_time seconds"
  if [ $ret -eq 0 ] ; then
    echo "Not all pods updated"
    exit 1
  fi
}

# Start functional test HTTP server.
# We skip the functional test additions for external provider for now, as they're specific
if [ "${KUBEVIRT_PROVIDER}" != "external" ] && [ "${CDI_SYNC}" == "test-infra" ]; then
  configure_storage
  _kubectl apply -f "./_out/manifests/bad-webserver.yaml"
  _kubectl apply -f "./_out/manifests/file-host.yaml"
  _kubectl apply -f "./_out/manifests/registry-host.yaml"
  # Imageio test service:
  _kubectl apply -f "./_out/manifests/imageio.yaml"
  # vCenter (VDDK) test service:
  _kubectl apply -f "./_out/manifests/vcenter.yaml"
  exit 0
fi

mkdir -p ./_out/tests
rm -f $OLD_CDI_VER_PODS $NEW_CDI_VER_PODS

seed_images

# Install CDI
install_cdi

#wait cdi crd is installed with timeout
wait_cdi_crd_installed $CDI_INSTALL_TIMEOUT

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
    else
      _kubectl apply -f "https://github.com/kubevirt/containerized-data-importer/releases/download/${VERSION}/cdi-cr.yaml"
      wait_cdi_available
    fi
    retry_counter=0
    kill_count=0
    while [[ $retry_counter -lt $CDI_UPGRADE_RETRY_COUNT ]] && [ "$operator_version" != "$VERSION" ]; do
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
    if [ $retry_counter -eq $CDI_UPGRADE_RETRY_COUNT ]; then
      echo "Unable to deploy to version $VERSION"
      cdi_obj=$(_kubectl get CDI -o yaml)
      echo $cdi_obj
      exit 1
    fi
    echo "Currently at version: $VERSION"
    wait_cdi_pods_updated
  done
  echo "Upgrading to latest"
  retry_counter=0
  _kubectl apply -f "./_out/manifests/release/cdi-operator.yaml"
  while [[ $retry_counter -lt $CDI_UPGRADE_RETRY_COUNT ]] && [ "$observed_version" != "latest" ]; do
    cdi_cr_phase=`_kubectl get CDI -o=jsonpath='{.items[*].status.phase}{"\n"}'`
    observed_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.observedVersion}{"\n"}'`
    target_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.targetVersion}{"\n"}'`
    operator_version=`_kubectl get CDI -o=jsonpath='{.items[*].status.operatorVersion}{"\n"}'`
    echo "Phase: $cdi_cr_phase, observedVersion: $observed_version, operatorVersion: $operator_version, targetVersion: $target_version"
    retry_counter=$((retry_counter + 1))
    _kubectl get pods -n $CDI_NAMESPACE
    sleep 5
  done
  if [ $retry_counter -eq $CDI_UPGRADE_RETRY_COUNT ]; then
	  echo "Unable to deploy to latest version"
	  cdi_obj=$(_kubectl get CDI -o yaml)
	  echo $cdi_obj
	  exit 1
  fi
  wait_cdi_available
  wait_cdi_pods_updated
  _kubectl patch crd cdis.cdi.kubevirt.io -p '{"spec": {"preserveUnknownFields":false}}'
else
  _kubectl apply -f "./_out/manifests/release/cdi-cr.yaml"
  wait_cdi_available
fi

# Grab all the CDI crds so we can check if they are structural schemas
cdi_crds=$(_kubectl get crd -l cdi.kubevirt.io -o jsonpath={.items[*].metadata.name})
crds=($cdi_crds)
operator_crds=$(_kubectl get crd -l operator.cdi.kubevirt.io -o jsonpath={.items[*].metadata.name})
crds+=($operator_crds)
check_structural_schema "${crds[@]}"
