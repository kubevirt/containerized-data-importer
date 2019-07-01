#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

function seed_images(){
  echo "seed_images is a noop for okd4.1"
}

function configure_local_storage() {
  #Check if we have already configured local storage, if so skip this step.
  NS="$(_kubectl get namespace local-storage --no-headers -o custom-columns=name:.metadata.name --ignore-not-found)"
  
  if [ "$NS" == "" ]; then
  	# local storage namespace doesn't exist, assume that we need to install local storage.
    for node in {"master-0", "worker-0"}
    do
      ./cluster-up/ssh.sh $node < cluster-sync/okd-4.1.0/create-local-storage-volumes.sh
    done

    #Create the local-storage namespace
    _kubectl new-project local-storage

    #Create the olm provisioner operator
    _kubectl create -f cluster-sync/okd-4.1.0/local-storage-operator.yaml
    set +e
    
    _kubectl get LocalVolume
    while [ $? == 1 ]
    do
    	sleep 5
    	_kubectl get LocalVolume
    done
    #Create the cr object.
    _kubectl create -f cluster-sync/okd-4.1.0/create-local-storage-cr.yaml

	SC="$(_kubectl get sc local-sc --no-headers -o custom-columns=name:.metadata.name --ignore-not-found)"
	while [ "$SC" == "" ]
	do
		sleep 5
		SC="$(_kubectl get sc local-sc --no-headers -o custom-columns=name:.metadata.name --ignore-not-found)"
	done
    #Set the default storage class.
    _kubectl patch storageclass local-sc -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
    set -e
  fi
}
