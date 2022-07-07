# Data Volume cloning with Smart-Cloning

## Introduction
Data Volumes (DV) can also be created by specifying a PVC as an input source. It will trigger a clone of the original PVC. See more details [here](datavolumes.md#pvc-source).

The CDI implementation of cloning a PVC is done with host assisted cloning by streaming the data from the source PVC and write to the new PVC.

In order to improve the performance of the cloning process, we introduced Smart-Cloning where snapshots are used.

## Smart-Cloning
CDI uses the feature of creating a PVC from snapshot in order to clone PVCs more efficiently when a CSI plugin with snapshot capabilities is available.

The yaml structure and annotations of the DV are not changed.

### Create PVC from snapshot
Kubernetes v1.12 introduced a feature enabling the creation of a PVC from a volume snapshot. See more details [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-snapshot-and-restore-volume-from-snapshot-support)

Note: To enable support for restoring a volume from a volume snapshot data source, enable the `VolumeSnapshotDataSource` feature gate on the apiserver and controller-manager.


### Flow description
Here is a description of the flow of the Smart-Cloning:

- DataVolume is created with a PVC source
- Check if Smart-Cloning is possible:
  * The source and target PVCs must be in the same Storage Class
  * There must be a Snapshot Class associated with the Storage Class
- If Smart-Cloning is possible:
  * Create a snapshot of the source PVC
  * Create a PVC from the created snapshot
  * Delete the snapshot
  * Expand the new PVC if requested size is larger than the snapshot
  * If the DataVolume is in a different namespace, "transfer" the PVC to the target namespace via [Namespace Transfer API](namespace-transfer.md)
- If Smart-Cloning is not possible:
  * Trigger a (slower) host-assisted clone

*Note: For some CSI driver when restoring from a snapshot, the new PVC size must equal the size of the PVC the snapshot was created from*

### Disabling smart cloning
If for some reason you don't want to use smart cloning and prefer using a host-assisted copy, you can disable smart cloning by editing the CDI object:
```bash
kubectl patch cdi cdi --type merge -p '{"spec":{"cloneStrategyOverride":"copy"}}'
```

To enable smart cloning again:
```bash
kubectl patch cdi cdi --type merge -p '{"spec":{"cloneStrategyOverride":"snapshot"}}'
```

### Smart clone from existing snapshot
Today each smart clone will create a temporary VolumeSnapshot.  
Some storage systems may be designed to scale better with a 1:N method where a single snapshot of the source PVC is used to create multiple clones.  
This is currently possibly (with some limitations described below) using an annotation on the DataVolume.

A suggested complete flow:
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: golden-snap-source
  namespace: golden-ns
spec:
  source:
      http:
         url: http://<SERVER>/Fedora-Cloud-Base-34-1.2.x86_64.qcow2
  pvc:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 8Gi
```

```yaml
apiVersion: snapshot.storage.k8s.io/v1
kind: VolumeSnapshot
metadata:
  name: golden-volumesnapshot
  namespace: golden-ns
spec:
  volumeSnapshotClassName: csi-rbdplugin-snapclass
  source:
    persistentVolumeClaimName: golden-snap-source
```

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  annotations:
    cdi.kubevirt.io/smartCloneFromExistingSnapshot: golden-ns/golden-volumesnapshot
  name: cloned-datavolume
  namespace: default
spec:
  source:
    pvc:
      namespace: golden-ns
      name: golden-snap-source
  pvc:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 8Gi
```

#### Known limitations
- Only cross-namespace (same namespace not supported)
- Spec.Source.PVC has to exist as a dummy PVC and must have a name that is not equal to the smartCloneFromExistingSnapshot
