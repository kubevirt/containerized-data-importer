#!/usr/bin/env bash

source cluster-sync/install.sh

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
  if [ "${EXTERNAL_PROVIDER}" == "openshift" ]; then
    echo "using external openshift cluster"
    echo "setting up ephemeral image registry"
    _kubectl apply -f ./cluster-sync/external/resources/docker-registry.yaml
    _kubectl wait deployment registry --for condition=available --timeout=180s
    while ! _kubectl get service docker-registry; do
      echo 'waiting for service'
      sleep 1
    done

    #_kubectl expose service docker-registry
    registry_host=$(_kubectl get route docker-registry --template='{{ .spec.host }}')
    echo "Registry host will be $registry_host"
    # now enable the insecure registry
    echo "Enabling insecure registry in openshift"
    _kubectl patch image.config.openshift.io/cluster --type merge -p '{"spec":{"registrySources":{"insecureRegistries":["'"$registry_host"'"]}}}'

    registry_port=5000
    registry_pod=$(_kubectl get pod -l app=docker-registry --no-headers -o custom-columns=:metadata.name)
    echo "Forwarding $registry_port to pod $registry_pod"
    _kubectl port-forward $registry_pod $registry_port:$registry_port > /dev/null&
    FORWARD_PID=$!

    DOCKER_PREFIX="localhost:$registry_port"
    MANIFEST_REGISTRY="$registry_host"
  fi
}

function configure_storage() {
  if [ "${EXTERNAL_PROVIDER}" == "openshift" ]; then
    if [[ $KUBEVIRT_STORAGE == "hpp" ]] ; then
      _kubectl apply -f ./cluster-sync/external/resources/machineconfig-worker.yaml
      echo "Installing hostpath provisioner storage, please ensure /var/hpvolumes exists and has the right SELinux labeling"
      HPP_RELEASE=$(curl -s https://github.com/kubevirt/hostpath-provisioner-operator/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
      _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
      _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
      _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_cr.yaml -n hostpath-provisioner
      _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc.yaml
      _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    fi
  else
    echo "Local storage not needed for external provider..."
  fi
}


