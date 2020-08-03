#!/usr/bin/env bash

source cluster-sync/ephemeral_provider.sh

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
  echo "External provider"
}

function configure_storage() {
  if [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
    _kubectl apply -f ./cluster-sync/external/resources/machineconfig-worker.yaml
    echo "Installing hostpath provisioner storage, please ensure /var/hpvolumes exists and has the right SELinux labeling"
    HPP_RELEASE=$(curl -s https://github.com/kubevirt/hostpath-provisioner-operator/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
    _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
    _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
    _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_cr.yaml -n hostpath-provisioner
    _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc.yaml
    _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
  elif [[ $KUBEVIRT_STORAGE == "ceph" ]] ; then
    echo "Installing hostpath provisioner storage"
    configure_ceph
  else
    echo "Local storage not needed for external provider..."
  fi
}


