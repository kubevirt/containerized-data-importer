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
  * The source and target PVCs must be in the same namespace
  * The source and target PVCs must be in the same Storage Class
  * There must be a Snapshot Class associated with the Storage Class
- If Smart-Cloning is possible:
  * Create a snapshot of the source PVC
  * Create a PVC from the created snapshot
  * Delete the snapshot
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
