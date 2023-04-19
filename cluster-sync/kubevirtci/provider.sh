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
  # Make sure no default storage class
  _kubectl annotate storageclasses --all storageclass.kubernetes.io/is-default-class-
  if [[ $KUBEVIRT_STORAGE == "rook-ceph-default" ]] ; then
    echo "Using builtin rook/ceph provisioner"
    if [[ $CEPH_WFFC == "true" ]] ; then
      _kubectl patch storageclass rook-ceph-block-wffc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    else
      _kubectl patch storageclass rook-ceph-block -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    fi
  elif [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
    echo "Installing hostpath provisioner storage"
    configure_hpp
  elif [[ $KUBEVIRT_STORAGE == "nfs" ]] ; then
    echo "Installing NFS CSI dynamic storage"
    configure_nfs_csi
  else
    echo "Using local volume storage"
    # Make sure local is default
    _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
  fi
}
