## Kubernetes Storage Overview

As a Containerized Data Importer user, I need to have a basic understanding of Kubernetes storage. The [Kubernetes storage documentation](https://kubernetes.io/docs/concepts/storage/) is very in depth, but we would like to highlight the basic building blocks of storage in Kubernetes.  The basic building block of storage in Kubernetes is Persistent Volumes ([PV](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)) which are a unit of storage. PVs define their size and other information needed to define the storage. When a user wants to use storage they define a Persistent Volume Claim ([PVC](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims)), which specifies the size of storage they want, and some other optional information. Kubernetes then tries to match an available PV to the PVC. If one is found, then the PVC is bound to the PV and the PVC becomes available. A pod can now mount the PVC using the pod spec when it starts.

In CDI we have seen three types of storage being used for different use cases.
* Manual hostPath based storage
* Local volume based storage
* Dynamic provisioned storage

Each one is useful for different use cases, which we will explain below. Note this document will likely become out of date as storage progresses at a rapid rate in Kubernetes.

### Manual hostPath based storage
Manual hostPath based storage is basically a mapping between a path on the host to a PV. This is the simplest form of storage and should be a good starting point. If you are trying CDI with [minikube](https://github.com/kubernetes/minikube) we will provide an example to get started.

#### minikube example
The contents here are directly taken from the [minikube persistent volume documentation](https://github.com/kubernetes/minikube/blob/master/docs/persistent_volumes.md) and we encourage you to read and understand that document.

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: pv0001
spec:
  accessModes:
    - ReadWriteOnce
  capacity:
    storage: 5Gi
  hostPath:
    path: /data/pv0001/
```

This will create a 5Gi sized PV, and now you are ready to apply one of the many [examples](../manifests/example) to this PV. If you run 

```bash
kubectl create -f  https://github.com/kubevirt/containerized-data-importer/blob/master/manifests/example/import-kubevirt-datavolume.yaml
```

It will create a Data Volume (DV) and import a cirros virtual disk image to that DV. If you try some of the other examples you might find that some of them require [scratch space](scratch-space.md). To create scratch space you simply need another PV available and CDI will automatically make use of it while importing. For cloning you will also need two PVs, one that contains the source data, and one that will be the target.

### Local volume based storage
Local volume storage is very similar to the manual storage scenario with a few differences. For one we will be using a storage class to group the PVs, secondly the storage class is defined as 'WaitForFirstConsumer' which allows kubernetes to schedule pods on any node before and then binding the PVC to a PV on the node that the pod that will be using the PVC runs on. This removes the constraints on the scheduler where a pod can run. The same issue still exists that once a pod has run, the PVC is tied to a particular node, and moving the data to a different node can be cumbersome, if not impossible. A great blog about the pros and cons of this approach can be found [here](https://kubernetes.io/blog/2019/04/04/kubernetes-1.14-local-persistent-volumes-ga/)

In the example we will be using bind mounted directories to serve as our volumes. This is straight forward to configure, but is obviously not the most performant way to create your volumes, each cluster will be different, and this example just serves as a guide.  

We grabbed this code from our kubevirtci images that we use for creating development clusters. The bash script creates 10 directories on each node and then bind mounts them on start up from /etc/fstab.
```bash
# Create local-volume directories
for i in {1..10}
do
  mkdir -p /var/local/kubevirt-storage/local-volume/disk${i}
  mkdir -p /mnt/local-storage/local/disk${i}
  echo "/var/local/kubevirt-storage/local-volume/disk${i} /mnt/local-storage/local/disk${i} none defaults,bind 0 0" >> /etc/fstab
done
chmod -R 777 /var/local/kubevirt-storage/local-volume
```

The following includes everything needed to create a local volume provisioner, the provisioner will create a PV for each disk mount point we created with the above script. We define the following kubernetes objects:

* StorageClass: The storage class used for the local volumes, we marked this as the default storage class, if your cluster has another default storage class, remove the annotation.
* ConfigMap:  The ConfigMap is what points the provisioners to the actual volume mounts so they can be used as PVs, if you have a different directory structure make the changes in the ConfigMap.
* ClusterRole/ClusterRoleBinding/Role/ServiceAccount: These are the basic accounts and roles needed to make the provisioners work with the appropriate permissions. We named the service account 'local-storage-admin' but you can name it anything you want.
* DaemonSet: The daemonset is what creates the provisioner pods, because it is a daemonset exactly one pod will run on each node. So if you have 4 nodes, there will be 4 provisioner pods.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
 name: local
 annotations:
  storageclass.kubernetes.io/is-default-class: "true"
provisioner: kubernetes.io/no-provisioner
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-storage-config
data:
  storageClassMap: |
    local:
       hostDir: /mnt/local-storage/local
       mountDir: /mnt/local-storage/local
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: local-storage-provisioner-pv-binding
subjects:
- kind: ServiceAccount
  name: local-storage-admin
  namespace: default
roleRef:
  kind: ClusterRole
  name: system:persistent-volume-provisioner
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: local-storage-provisioner-node-clusterrole
rules:
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: local-storage-provisioner-node-binding
subjects:
- kind: ServiceAccount
  name: local-storage-admin
  namespace: default
roleRef:
  kind: ClusterRole
  name: local-storage-provisioner-node-clusterrole
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: local-storage-provisioner-jobs-role
rules:
- apiGroups:
    - 'batch'
  resources:
    - jobs
  verbs:
    - '*'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: local-storage-provisioner-jobs-rolebinding
subjects:
- kind: ServiceAccount
  name: local-storage-admin
roleRef:
  kind: Role
  name: local-storage-provisioner
  apiGroup: rbac.authorization.k8s.io
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: local-storage-admin
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: local-volume-provisioner
  labels:
    app: local-volume-provisioner
spec:
  selector:
    matchLabels:
      app: local-volume-provisioner
  template:
    metadata:
      labels:
        app: local-volume-provisioner
    spec:
      serviceAccountName: local-storage-admin
      containers:
        - image: "quay.io/external_storage/local-volume-provisioner:v2.1.0"
          name: provisioner
          securityContext:
            privileged: true
          env:
          - name: MY_NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
          - name: MY_NAMESPACE
            valueFrom:
              fieldRef:
                fieldPath: metadata.namespace
          - name: JOB_CONTAINER_IMAGE
            value: "quay.io/external_storage/local-volume-provisioner:v2.1.0"
          volumeMounts:
            - mountPath: /etc/provisioner/config
              name: provisioner-config
              readOnly: true
            - mountPath: /mnt/local-storage
              name: local-storage
              mountPropagation: "HostToContainer"
      volumes:
        - name: provisioner-config
          configMap:
            name: local-storage-config
        - name: local-storage
          hostPath:
            path: /mnt/local-storage
```

### Dynamic provisioning storage
The difference between dynamic provisioning and static provisioning is that PVs are created on demand. If a user requests a 5Gi PVC, a matching PV is created (if possible) and bound to the PVC. There are many dynamic provisioned storage solutions available like gluster, ceph and hostPath based dynamic provisioning.

 One last thing to highlight is the difference between shared and non-shared storage. The manual and local volume based storage is not shared between nodes, and thus if a node goes down recovering your data will be a lot harder than if you use shared storage like gluster or ceph. Another benefit of shared storage is that pods can move freely between nodes, so if a node becomes saturated and it starts to kill pods, they can easily be started on another node and use the same storage, this is not possible without shared storage.
