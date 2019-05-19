#!/usr/bin/env bash

function _kubectl(){
  kubectl "$@"
}

function seed_images(){
  echo "seed_images is a noop for external provider"
}

function verify() {
  echo "Verify not needed for external provider"
}


function up() {
  echo "using external provider"
}

function configure_local_storage() {
  echo "Local storage not needed for external provider..."
}

function install_cdi {
  _kubectl apply -f "./_out/manifests/release/cdi-operator.yaml" 
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

