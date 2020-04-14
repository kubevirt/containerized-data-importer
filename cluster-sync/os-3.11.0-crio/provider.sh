#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

function fix_failed_sdn_pods() {
   broken_pods=$(_kubectl get pods -n openshift-sdn -o jsonpath={.items[?\(@.status.containerStatuses[0].ready==false\)].metadata.name})
   for pod in ${broken_pods[@]}; do
     echo "Fixing broken pod $pod"
     _kubectl delete pod $pod -n openshift-sdn
   done
}

