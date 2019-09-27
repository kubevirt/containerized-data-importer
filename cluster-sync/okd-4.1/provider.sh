#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

function seed_images(){
  echo "seed_images is a noop for okd4.1"
}

function configure_local_storage() {
  retry_counter=0
  sc=`_kubectl get sc local -o=jsonpath="{.metadata.name}"`
  all_sc=`_kubectl get sc`
  echo "All storage classes:"
  echo "${all_sc}"
  while [[ $retry_counter -lt 60 ]] && [ "$sc" != "local" ]; do
    sc=`_kubectl get sc local -o=jsonpath="{.metadata.name}"`
    retry_counter=$((retry_counter + 1))
    echo "Sleep 5s, waiting for local storage class, current sc=[$sc]"
    sleep 5
    all_sc=`_kubectl get sc`
    echo "All storage classes:"
    echo "${all_sc}"
  done

  #Set the default storage class. If the above timed out, this will fail and abort the sync
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

