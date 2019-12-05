#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

function seed_images(){
  echo "seed_images is a noop for okd4.3"
}

ROOK_CEPH_VERSION=${ROOK_CEPH_VERSION:-v1.1.4}

function configure_storage() {
  #Make sure local is not default
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'

  #Configure ceph storage.
  set +e
  _kubectl create -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/common.yaml
  _kubectl create -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/operator-openshift.yaml
  _kubectl create -f ./cluster-sync/${KUBEVIRT_PROVIDER}/rook_ceph.yaml
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
  _kubectl create -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/csi/rbd/snapshotclass.yaml
  set -e
}

