#!/usr/bin/env bash
set -e

source cluster-sync/install.sh

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
      ./cluster-up/ssh.sh "node$(printf "%02d" ${i})" "echo \"${container}\" | xargs \-\-max-args=1 sudo docker pull"
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
  HPP_RELEASE=$(curl -s https://github.com/kubevirt/hostpath-provisioner-operator/releases/latest | grep -o "v[0-9]\.[0-9]*\.[0-9]*")
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/namespace.yaml
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/operator.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/hostpathprovisioner_cr.yaml -n hostpath-provisioner
  _kubectl apply -f https://github.com/kubevirt/hostpath-provisioner-operator/releases/download/$HPP_RELEASE/storageclass-wffc.yaml
  _kubectl patch storageclass hostpath-provisioner -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_ceph() {
  #Configure ceph storage.
  _kubectl apply -f ./cluster-sync/external-snapshotter
  _kubectl apply -f ./cluster-sync/rook-ceph/common.yaml
  if _kubectl get securitycontextconstraints; then
    _kubectl apply -f ./cluster-sync/rook-ceph/scc.yaml
  fi
  _kubectl apply -f ./cluster-sync/rook-ceph/operator.yaml
  _kubectl apply -f ./cluster-sync/rook-ceph/cluster.yaml
  _kubectl apply -f ./cluster-sync/rook-ceph/pool.yaml

  # wait for ceph
  until _kubectl get cephblockpools -n rook-ceph replicapool -o jsonpath='{.status.phase}' | grep Ready; do
      ((count++)) && ((count == 120)) && echo "Ceph not ready in time" && exit 1
      if ! ((count % 6 )); then
        _kubectl get pods -n rook-ceph
      fi
      echo "Waiting for Ceph to be Ready, sleeping 5s and rechecking"
      sleep 5
  done

  _kubectl patch storageclass rook-ceph-block -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_nfs() {
  #Configure static nfs service and storage class, so we can create NFS PVs during test run.
  _kubectl apply -f ./cluster-sync/nfs/nfs-sc.yaml
  _kubectl apply -f ./cluster-sync/nfs/nfs-service.yaml -n $CDI_NAMESPACE
  _kubectl apply -f ./cluster-sync/nfs/nfs-server.yaml -n $CDI_NAMESPACE
  _kubectl patch storageclass nfs -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}

function configure_ember_lvm() {
  _kubectl apply -f ./cluster-sync/external-snapshotter

  _kubectl apply -f ./cluster-sync/ember/loop_back.yaml -n ember-csi-lvm
  set +e

  loopdeviceNode=$(_kubectl get pods -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={.items[0].spec.nodeName})
  echo "loop back device node [$loopdeviceNode]"
  retry_counter=0
  while [[ $loopdeviceNode == "" ]] && [[ $retry_counter -lt 60 ]]; do
    retry_counter=$((retry_counter + 1))
    loopdeviceNode=$(_kubectl get pods -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={.items[0].spec.nodeName})
    echo "Sleep 1s, waiting for loopback device pod node to be found [$loopdeviceNode]"
    sleep 1
  done
  echo "Loop back device pod is running on node $loopdeviceNode"
  podIp=$(_kubectl get pods -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={.items[0].status.podIP})
  echo "loopback podIP: $podIp"
  retry_counter=0
  while [[ $podIp == "" ]] && [[ $retry_counter -lt 60 ]]; do
    retry_counter=$((retry_counter + 1))
    sleep 1
    podIp=$(_kubectl get pods -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={.items[0].status.podIP})
    echo "loopback podIP: $podIp"
  done

  success=$(_kubectl get pod -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={".items[0].status.phase"})
  retry_counter=0
  while [[ $success != "Succeeded" ]] && [[ $retry_counter -lt 60 ]]; do
    retry_counter=$((retry_counter + 1))
    sleep 1
    success=$(_kubectl get pod -n ember-csi-lvm -l app=loop-back-lvm -o=jsonpath={".items[0].status.phase"})
  done
  echo "Loop back device available, starting ember csi controller"
  _kubectl apply -f ./cluster-sync/ember/ember-csi-lvm.yaml -n ember-csi-lvm

  cat <<EOF | _kubectl apply -n ember-csi-lvm -f -
kind: StatefulSet
apiVersion: apps/v1
metadata:
  name: csi-controller
  labels:
    app: embercsi
    embercsi_cr: lvm
spec:
  serviceName: csi-controller
  replicas: 1
  selector:
    matchLabels:
      app: embercsi
      embercsi_cr: lvm
  template:
    metadata:
      labels:
        app: embercsi
        embercsi_cr: lvm
    spec:
      nodeSelector:
        kubernetes.io/hostname: $loopdeviceNode
      serviceAccount: csi-controller-sa
      # iSCSI only the very latest Open-iSCSI supports namespaces
      hostNetwork: true
      # Required by multipath detach (some drivers clone volumes dd-ing)
      hostIPC: true
      containers:
      - name: external-provisioner
        image: quay.io/k8scsi/csi-provisioner:v1.5.0
        args:
        - --v=5
        - --provisioner=ember-csi.io
        - --csi-address=/csi-data/csi.sock
        - --feature-gates=Topology=true
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi-data
          name: socket-dir
      - name: external-attacher
        image: quay.io/k8scsi/csi-attacher:v2.1.0
        args:
        - --v=5
        - --csi-address=/csi-data/csi.sock
        - --timeout=120s
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi-data
          name: socket-dir
      - name: external-snapshotter
        image: quay.io/k8scsi/csi-snapshotter:v2.1.0
        args:
        - --v=5
        - --csi-address=/csi-data/csi.sock
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi-data
          name: socket-dir
      - name: external-resizer
        image: quay.io/k8scsi/csi-resizer:v0.3.0
        args:
        - --v=5
        - --csi-address=/csi-data/csi.sock
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /csi-data
          name: socket-dir
      - name: csi-driver
        image: mhenriks/ember-csi:centos8
        terminationMessagePath: /tmp/termination-log
        # command: ["tail"]
        # args: ["-f", "/dev/null"]
        imagePullPolicy: Always
        securityContext:
          privileged: true
          allowPrivilegeEscalation: true
        env:
        - name: PYTHONUNBUFFERED
          value: '0'
        - name: CSI_ENDPOINT
          value: unix:///csi-data/csi.sock
        - name: KUBE_NODE_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: spec.nodeName
        - name: X_CSI_SPEC_VERSION
          value: v1.1
        - name: CSI_MODE
          value: controller
        - name: X_CSI_PERSISTENCE_CONFIG
          value: '{"storage":"crd"}'
        - name: X_CSI_EMBER_CONFIG
          value: '{"debug":true,"enable_probe":true}'
        - name: X_CSI_TOPOLOGIES
          value: '[{"iscsi":"true"}]'
        - name: X_CSI_BACKEND_CONFIG
          value: '{"target_protocol":"iscsi","target_ip_address":"$podIp","name":"lvm","driver":"LVMVolume","volume_group":"ember-volumes","target_helper":"lioadm","multipath":false}'
        livenessProbe:
          exec:
            command:
            - ember-liveness
          initialDelaySeconds: 120
          periodSeconds: 90
          timeoutSeconds: 60
        volumeMounts:
        - name: socket-dir
          mountPath: /csi-data
        - name: iscsi-dir
          mountPath: /etc/iscsi
          mountPropagation: Bidirectional
        - name: dev-dir
          mountPath: /dev
          mountPropagation: Bidirectional
        - name: lvm-dir
          mountPath: /etc/lvm
          mountPropagation: Bidirectional
        - name: lvm-lock
          mountPath: /var/lock/lvm
          mountPropagation: Bidirectional
        - name: multipath-dir
          mountPath: /etc/multipath
          mountPropagation: Bidirectional
        - name: multipath-conf
          mountPath: /etc/multipath.conf
          mountPropagation: HostToContainer
        - name: modules-dir
          mountPath: /lib/modules
          mountPropagation: HostToContainer
        - name: localtime
          mountPath: /etc/localtime
          mountPropagation: HostToContainer
        - name: udev-data
          mountPath: /run/udev
          mountPropagation: HostToContainer
        - name: lvm-run
          mountPath: /run/lvm
          mountPropagation: HostToContainer
        # Required to preserve the node targets between restarts
        - name: iscsi-info
          mountPath: /var/lib/iscsi
          mountPropagation: Bidirectional
        # In a real deployment we should be mounting container's
        # /var/lib-ember-csi on the host
      - name: csc
        image: embercsi/csc:v1.1.0
        command: ["tail"]
        args: ["-f", "/dev/null"]
        securityContext:
          privileged: true
        env:
          - name: CSI_ENDPOINT
            value: unix:///csi-data/csi.sock
        volumeMounts:
          - name: socket-dir
            mountPath: /csi-data
      volumes:
      - name: socket-dir
        emptyDir:
      # Some backends do create volume from snapshot by attaching and dd-ing
      - name: iscsi-dir
        hostPath:
          path: /etc/iscsi
          type: Directory
      - name: dev-dir
        hostPath:
          path: /dev
      - name: lvm-dir
        hostPath:
          path: /etc/lvm
          type: Directory
      - name: lvm-lock
        hostPath:
          path: /var/lock/lvm
      - name: multipath-dir
        hostPath:
          path: /etc/multipath
      - name: multipath-conf
        hostPath:
          path: /etc/multipath.conf
      - name: modules-dir
        hostPath:
          path: /lib/modules
      - name: localtime
        hostPath:
          path: /etc/localtime
      - name: udev-data
        hostPath:
          path: /run/udev
      - name: lvm-run
        hostPath:
          path: /run/lvm
      - name: iscsi-info
        hostPath:
          path: /var/lib/iscsi
EOF

  echo "Waiting for ember-csi controller to be running"
  running=$(_kubectl get pod -n ember-csi-lvm csi-controller-0 -o=jsonpath={".status.phase"})
  retry_counter=0
  while [[ $running != "Running" ]] && [[ $retry_counter -lt 60 ]]; do
    retry_counter=$((retry_counter + 1))
    sleep 5
    running=$(_kubectl get pod -n ember-csi-lvm csi-controller-0 -o=jsonpath={".status.phase"})
  done

  _kubectl apply -f ./cluster-sync/ember/snapshot-class.yaml

  _kubectl patch storageclass ember-csi-lvm -p '{"metadata": {"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}'
}
