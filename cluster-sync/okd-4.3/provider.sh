#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

function seed_images(){
  echo "seed_images is a noop for okd4.3"
}

function configure_storage() {
    _kubectl patch deployment -n openshift-machine-config-operator machine-config-operator -p '{"spec": {"replicas": 1}}'
    _kubectl patch deployment -n openshift-machine-config-operator etcd-quorum-guard -p '{"spec": {"replicas": 1}}'
  wait_machine_config_pool "True"
  #Make sure local is not default
  set +e
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
  set -e
  if [[ $KUBEVIRT_STORAGE == "ceph" ]] ; then
    echo "Installing ceph storage"
    configure_ceph
  elif [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
    echo "Installing hostpath provisioner storage"
    configure_hpp
    _kubectl apply -f ./cluster-sync/${KUBEVIRT_PROVIDER}/hpp-worker-mc.yaml
    # This will seem odd, but I am waiting on false to be sure the machine config update started before checking if it finished
    # otherwise it will seem finished before it started. May be better to check for current the rendered version vs the requested
    # version instead.
    wait_machine_config_pool "False"
    wait_machine_config_pool "True"
    # The machine config cannot update the master because that would make it lose the control plane, if we have multiple masters
    # then we could update them using the machine config. So manually setting the master with ssh.
    master_name=$(_kubectl get nodes -l node-role.kubernetes.io/master -o=jsonpath='{range .items[*]}{.metadata.name}')
    echo "Relabeling SELinux on master $master_name"
    ./cluster-up/ssh.sh $master_name "sudo chcon -t container_file_t -R /var/hpvolumes"
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
  _kubectl create -f https://raw.githubusercontent.com/rook/rook/$ROOK_CEPH_VERSION/cluster/examples/kubernetes/ceph/operator-openshift.yaml
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

function wait_machine_config_pool() {
  #Wait for machineconfigpool to be updated.
  set +e
  updated=$(_kubectl get machineconfigpool worker -o=jsonpath="{.status.conditions[?(@.type=='Updated')].status}{\"\n\"}")
  echo "Is updated? $updated"
  until [ $updated == "$1" ]; do
    echo "status.conditions.updated doesn't match requested value $1"
    sleep 10
    updated=$(_kubectl get machineconfigpool worker -o=jsonpath="{.status.conditions[?(@.type=='Updated')].status}{\"\n\"}")
  done
  echo "Is updated? $updated"
  set -e
}