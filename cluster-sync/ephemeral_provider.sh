#!/usr/bin/env bash
set -e

source cluster-sync/install.sh
source hack/common-funcs.sh

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
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "echo \"${container}\" | xargs \-\-max-args=1 sudo crictl pull"
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
  for i in $(seq 1 ${KUBEVIRT_NUM_NODES}); do
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo mkdir -p /var/hpvolumes"
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo chcon -t container_file_t -R /var/hpvolumes"
  done
  HPP_RELEASE=$(get_latest_release "kubevirt/hostpath-provisioner-operator")
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
  #install cert-manager
  _kubectl apply -f https://github.com/jetstack/cert-manager/releases/download/v1.6.1/cert-manager.yaml
  _kubectl wait --for=condition=available -n cert-manager --timeout=120s --all deployments
  _kubectl apply -f https://raw.githubusercontent.com/kubevirt/hostpath-provisioner-operator/main/deploy/webhook.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
  echo "Waiting for it to be ready"
  _kubectl rollout status -n hostpath-provisioner deployment/hostpath-provisioner-operator --timeout=120s

  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_legacy_cr.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc-legacy.yaml
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc-legacy-csi.yaml
  echo "Waiting for hostpath provisioner to be available"
  _kubectl wait hostpathprovisioners.hostpathprovisioner.kubevirt.io/hostpath-provisioner --for=condition=Available --timeout=480s
}

function configure_hpp_classic() {
  # Configure hpp and default to classic non-csi hostpath-provisioner
  configure_hpp
  _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_hpp_csi() {
  # Configure hpp and default to hostpath-csi
  configure_hpp
   _kubectl patch storageclass hostpath-csi -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_nfs() {
  #Configure static nfs service and storage class, so we can create NFS PVs during test run.
  _kubectl apply -f ./cluster-sync/nfs/nfs-sc.yaml
  _kubectl apply -f ./cluster-sync/nfs/nfs-service.yaml -n $CDI_NAMESPACE
  _kubectl apply -f ./cluster-sync/nfs/nfs-server.yaml -n $CDI_NAMESPACE
  _kubectl patch storageclass nfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_nfs_csi() {
  #Configure dynamic NFS provisioner which is deployed by kubevirtci (https://github.com/kubernetes-csi/csi-driver-nfs)
  _kubectl patch storageclass nfs-csi -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}
