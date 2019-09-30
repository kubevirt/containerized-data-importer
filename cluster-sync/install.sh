#!/usr/bin/env bash

set -e
source ./cluster-sync/install-config.sh

function install_cdi {
  if [ ! -z $UPGRADE_FROM ]; then
    curl -L "https://github.com/kubevirt/containerized-data-importer/releases/download/$UPGRADE_FROM/cdi-operator.yaml" --output cdi-operator.yaml
    sed -i "0,/name: cdi/{s/name: cdi/name: $CDI_NAMESPACE/}" cdi-operator.yaml
    sed -i "s/namespace: cdi/namespace: $CDI_NAMESPACE/g" cdi-operator.yaml
    echo $(cat cdi-operator.yaml)
    _kubectl apply -f cdi-operator.yaml
  else
    _kubectl apply -f "./_out/manifests/release/cdi-operator.yaml"
  fi
}

function wait_cdi_crd_installed {
  timeout=$1
  crd_defined=0
  while [ $crd_defined -eq 0 ] && [ $timeout > 0 ]; do
      crd_defined=$(_kubectl get customresourcedefinition| grep cdis.cdi.kubevirt.io | wc -l)
      sleep 1
      timeout=$(($timeout-1))
  done

  #In case CDI crd is not defined after 120s - throw error
  if [ $crd_defined -eq 0 ]; then
     echo "ERROR - CDI CRD is not defined after timeout"
     exit 1
  fi  
}

