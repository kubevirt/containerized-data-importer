#!/usr/bin/env bash

set -e
source ./cluster-sync/install-config.sh
source ./cluster-sync/${KUBEVIRT_PROVIDER}/install.sh

function install_cdi_olm {
  #Install CDI via OLM
  _kubectl create ns $NAMESPACE
  _kubectl apply -f _out/manifests/release/olm/operatorgroup.yaml

  if [ -z "$CDI_OLM_MANIFESTS_CATALOG_SRC" ]; then 
      echo "ERROR - OLM installation requires CDI_OLM_MANIFESTS_CATALOG_SRC to be set"
      exit -1
  fi

  if [ -z "$CDI_OLM_MANIFESTS_SUBSCRIPTION" ]; then 
      echo "ERROR - OLM installation requires CDI_OLM_MANIFESTS_SUBSCRIPTION to be set"
      exit -1
  fi

  _kubectl apply -f $CDI_OLM_MANIFESTS_CATALOG_SRC
  _kubectl apply -f $CDI_OLM_MANIFESTS_SUBSCRIPTION
}

function install_cdi_operator {
  _kubectl apply -f "./_out/manifests/release/cdi-operator.yaml" 
}


function install_cdi {
    case "${CDI_INSTALL}" in
    "${CDI_INSTALL_OPERATOR}")
        install_cdi_operator
        ;;
    "${CDI_INSTALL_OLM}")
        install_cdi_olm
        ;;
    esac
}


function wait_cdi_crd_installed {
  timeout=$1
  crd_defined=0
  while [ $crd_defined -eq 0 ] && [ $timeout > 0 ]; do
      crd_defined=$(_kubectl get customresourcedefinition| grep cdis.cdi.kubevirt.io | wc -l)
      sleep 1
      timeout=timeout-1
  done

  #In case CDI crd is not defined after 120s - throw error
  if [ $timeout \< 1 ]; then
     echo "ERROR - CDI CRD is not defined after timeout"
     exit 1
  fi  

}

