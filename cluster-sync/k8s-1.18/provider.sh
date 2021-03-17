#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

function configure_storage() {
  #Make sure local is not default
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
  if [[ $KUBEVIRT_STORAGE == "rook-ceph-default" ]] ; then
    echo "Using builtin rook/ceph provisioner"
  # ceph will be deprecated once changes to test lanes merged
  elif [[ $KUBEVIRT_STORAGE == "ceph" ]] ; then
    echo "Installing ceph storage"
    configure_ceph
  elif [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
    echo "Installing hostpath provisioner storage"
    configure_hpp
  elif [[ $KUBEVIRT_STORAGE == "nfs" ]] ; then
    echo "Installing NFS static storage"
    configure_nfs
  elif [[ $KUBEVIRT_STORAGE == "ember_lvm" ]] ; then
    echo "Installing ember csi lvm storage"
    configure_ember_lvm
  else
    echo "Using local volume storage"
    #Make sure local is not default
    _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
  fi
}
