#!/usr/bin/env bash
set -e

source cluster-sync/install.sh

ROOK_CEPH_VERSION=${ROOK_CEPH_VERSION:-v1.1.4}

function seed_images(){
  container=""
  container_alias=""
  images="${@:-${DOCKER_IMAGES}}"
  for arg in $images; do
      name=$(basename $arg)
      container="${container} registry:5000/${name}:${DOCKER_TAG}"
  done

  # We don't need to seed the nodes, but in the case of the default dev setup we'll just leave this here
  for i in $(seq 1 ${KUBEVIRT_NUM_NODES}); do
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "echo \"${container}\" | xargs \-\-max-args=1 sudo docker pull"
      # Temporary until image is updated with provisioner that sets this field
      # This field is required by buildah tool
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo sysctl \-w user.max_user_namespaces=1024"
  done

}

function verify() {
  echo 'Wait until all nodes are ready'
  until [[ $(_kubectl get nodes --no-headers | wc -l) -eq $(_kubectl get nodes --no-headers | grep " Ready" | wc -l) ]]; do
    sleep 1
  done
  echo "cluster node are ready!"
}

function configure_storage() {
  echo "Storage already configured ..."
}

function configure_hpp() {
  HPP_RELEASE=$(curl -s https://github.com/kubevirt/hostpath-provisioner-operator/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_cr.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc.yaml
  _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
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

function configure_nfs() {
  #Configure static nfs service and storage class, so we can create NFS PVs during test run.
  _kubectl apply -f ./cluster-sync/nfs/nfs-sc.yaml
  _kubectl apply -f ./cluster-sync/nfs/nfs-service.yaml -n $CDI_NAMESPACE
  _kubectl apply -f ./cluster-sync/nfs/nfs-server.yaml -n $CDI_NAMESPACE
  _kubectl patch storageclass nfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}