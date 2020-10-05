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

# Need to set the DOCKER_PREFIX appropriately in the call to `make docker push`, otherwise make will just pass in the default `kubevirt`
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
    while [[ $available != "True" ]] && [[ $retry_counter -lt ${CDI_AVAILABLE_TIMEOUT} ]]; do
      wait_time=$((wait_time + 5))
      sleep 5
      available=$(_kubectl get cdi cdi -o jsonpath={.status.conditions[0].status})
      fix_failed_sdn_pods
    done
  else
    _kubectl wait cdis.cdi.kubevirt.io/${CR_NAME} --for=condition=Available --timeout=${CDI_AVAILABLE_TIMEOUT}s
  fi
}

seed_images

# Install CDI
install_cdi

#wait cdi crd is installed with timeout
wait_cdi_crd_installed $CDI_INSTALL_TIMEOUT

# Grab all the CDI crds so we can check if they are structural schemas
cdi_crds=$(_kubectl get crd -l cdi.kubevirt.io -o jsonpath={.items[*].metadata.name})
crds=($cdi_crds)
operator_crds=$(_kubectl get crd -l operator.cdi.kubevirt.io -o jsonpath={.items[*].metadata.name})
crds+=($operator_crds)
check_structural_schema "${crds[@]}"

configure_storage

# Start functional test HTTP server.
# We skip the functional test additions for external provider for now, as they're specific
_kubectl apply -f "./_out/manifests/bad-webserver.yaml"
_kubectl apply -f "./_out/manifests/file-host.yaml"
_kubectl apply -f "./_out/manifests/registry-host.yaml"
# Imageio test service:
_kubectl apply -f "./_out/manifests/imageio.yaml"
# vCenter (VDDK) test service:
_kubectl apply -f "./_out/manifests/vcenter.yaml"
