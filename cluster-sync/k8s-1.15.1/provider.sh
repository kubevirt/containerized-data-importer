#!/usr/bin/env bash
set -e
source cluster-sync/ephemeral_provider.sh

num_nodes=${KUBEVIRT_NUM_NODES:-1}
mem_size=${KUBEVIRT_MEMORY_SIZE:-5120M}

re='^-?[0-9]+$'
if ! [[ $num_nodes =~ $re ]] || [[ $num_nodes -lt 1 ]] ; then
    num_nodes=1
fi

if [[ $KUBEVIRT_PROVIDER_EXTRA_ARGS =~ "--enable-ceph" ]]; then
  echo "Switching default storage class to ceph"
  # Switch the default storage class to ceph
  _kubectl patch storageclass local -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}'
  # Make sure the SC is configured to use xfs instead of ext4
  _kubectl delete storageclass csi-rbd
  cat <<EOF | _kubectl apply -f -
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: csi-rbd
parameters:
  adminid: admin
  csi.storage.k8s.io/node-publish-secret-name: csi-rbd-secret
  csi.storage.k8s.io/node-publish-secret-namespace: default
  csi.storage.k8s.io/provisioner-secret-name: csi-rbd-secret
  csi.storage.k8s.io/provisioner-secret-namespace: default
  imageFeatures: layering
  imageFormat: "2"
  monitors: 192.168.66.2
  multiNodeWritable: enabled
  pool: rbd
  fsType: xfs
provisioner: csi-rbdplugin
reclaimPolicy: Delete
volumeBindingMode: Immediate
EOF
fi

