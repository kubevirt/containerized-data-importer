#!/usr/bin/env bash
set -e
source cluster/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

image="os-3.11.0-crio@sha256:3f11a6f437fcdf2d70de4fcc31e0383656f994d0d05f9a83face114ea7254bc0"

function up() {
    # If on a developer setup, expose ocp on 8443, so that the openshift web console can be used (the port is important because of auth redirects)
    if [ -z "${JOB_NAME}" ]; then
        KUBEVIRT_PROVIDER_EXTRA_ARGS="${KUBEVIRT_PROVIDER_EXTRA_ARGS} --ocp-port 8443"
    fi

    $gocli run --random-ports --reverse --nodes ${num_nodes} --memory ${mem_size} --background kubevirtci/${image} ${KUBEVIRT_PROVIDER_EXTRA_ARGS}
    cluster_port=$($gocli ports ocp | tr -d '\r')
    $gocli scp /etc/origin/master/admin.kubeconfig - > ./cluster/.kubeconfig
    $gocli ssh node01 -- sudo cp /etc/origin/master/admin.kubeconfig ~vagrant/
    $gocli ssh node01 -- sudo chown vagrant:vagrant ~vagrant/admin.kubeconfig

    # Copy oc tool and configuration file
    $gocli scp /usr/bin/oc - >./cluster/.kubectl
    chmod u+x ./cluster/.kubectl
    $gocli scp /etc/origin/master/admin.kubeconfig - > ./cluster/.kubeconfig
    # Update Kube config to support unsecured connection
    export KUBECONFIG=./cluster/.kubeconfig
    ./cluster/.kubectl config set-cluster node01:8443 --server=https://127.0.0.1:$cluster_port
    ./cluster/.kubectl config set-cluster node01:8443 --insecure-skip-tls-verify=true
}