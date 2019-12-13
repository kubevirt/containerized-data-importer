#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

ROOK_CEPH_VERSION=${ROOK_CEPH_VERSION:-v1.1.4}

function configure_storage() {
  #Make sure local is not default
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
  if [[ $KUBEVIRT_STORAGE == "ceph" ]] ; then
    echo "Installing ceph storage"
    configure_ceph
  elif [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
    echo "Installing hostpath provisioner storage"
    configure_hpp
  else
    echo "Using local volume storage"
    #Make sure local is not default
    _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
  fi
}

function configure_ceph() {
  #Configure ceph storage.
  set +e
  _kubectl apply -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/common.yaml
  _kubectl apply -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/operator.yaml
  _kubectl apply -f ./cluster-sync/${KUBEVIRT_PROVIDER}/rook_ceph.yaml
  cat <<EOF | _kubectl apply -f -
apiVersion: ceph.rook.io/v1
kind: CephBlockPool
metadata:
  name: replicapool
  namespace: rook-ceph
spec:
  failureDomain: host
  replicated:
    size: $KUBEVIRT_NUM_NODES
EOF

  _kubectl create -f ./cluster-sync/${KUBEVIRT_PROVIDER}/ceph_sc.yaml
  set +e
  retry_counter=0
  _kubectl get VolumeSnapshotClass
   while [[ $? -ne "0" ]] && [[ $retry_counter -lt 60 ]]; do
    echo "Sleep 5s, waiting for VolumeSnapshotClass CRD"
   sleep 5
   _kubectl get VolumeSnapshotClass
  done
  echo "VolumeSnapshotClass CRD available, creating snapshot class"
  _kubectl apply -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/csi/rbd/snapshotclass.yaml
  set -e
}

function configure_hpp() {
  HPP_RELEASE=$(curl -s https://github.com/kubevirt/hostpath-provisioner-operator/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_cr.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc.yaml
  _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}
