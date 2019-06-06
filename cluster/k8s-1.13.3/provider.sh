#!/usr/bin/env bash
set -e
source ./cluster/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

image="k8s-1.13.3@sha256:9c2b78e11c25b3fd0b24b0ed684a112052dff03eee4ca4bdcc4f3168f9a14396"

function up() {
	$gocli run --enable-ceph --random-ports --nodes ${num_nodes} --memory ${mem_size} --background kubevirtci/${image}
    cluster_port=$($gocli ports k8s | tr -d '\r')
    $gocli scp /usr/bin/kubectl - > ./cluster/.kubectl
    chmod u+x ./cluster/.kubectl
    $gocli scp /etc/kubernetes/admin.conf - > ./cluster/.kubeconfig
    export KUBECONFIG=./cluster/.kubeconfig
    ./cluster/.kubectl config set-cluster kubernetes --server=https://127.0.0.1:$cluster_port
    ./cluster/.kubectl config set-cluster kubernetes --insecure-skip-tls-verify=true
}