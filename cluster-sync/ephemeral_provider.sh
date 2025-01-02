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
      until ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "echo \"${container}\" | xargs \-\-max-args=1 sudo crictl pull"; do sleep 1; done
      # Temporary until image is updated with provisioner that sets this field
      # This field is required by buildah tool
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo sysctl \-w user.max_user_namespaces=1024"
  done

}

# For the Kind provider, we need to configure hostname resolution for the local image registry in the CoreDNS service.
# This ensures that local container images can be successfully pulled into Kubernetes pods during certain e2e tests.
function setup_hostname_resolution_for_registry {
 host_name="registry"
 host_ip=$(${CDI_CRI} inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' $(${CDI_CRI} ps|grep registry|awk '{print $1}'))
 _kubectl patch configmap coredns \
   -n kube-system \
   --type merge \
   -p "{\"data\":{\"Corefile\":\".:53 {\n    errors\n    health {\n       lameduck 5s\n    }\n    ready\n    kubernetes cluster.local in-addr.arpa ip6.arpa {\n       pods insecure\n       fallthrough in-addr.arpa ip6.arpa\n       ttl 30\n    }\n    prometheus :9153\n    forward . /etc/resolv.conf {\n       max_concurrent 1000\n    }\n    cache 30\n    loop\n    reload\n    loadbalance\n    hosts {\n        $host_ip   $host_name\n        fallthrough\n    }\n}\"}}"
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
  if [[ $KUBEVIRT_PROVIDER =~ kind.* ]]; then
      ./cluster-up/ssh.sh ${KUBEVIRT_PROVIDER}-control-plane mkdir -p /var/hpvolumes
  else
      for i in $(seq 1 ${KUBEVIRT_NUM_NODES}); do
         ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo mkdir -p /var/hpvolumes"
         ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "sudo chcon -t container_file_t -R /var/hpvolumes"
      done
  fi
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
