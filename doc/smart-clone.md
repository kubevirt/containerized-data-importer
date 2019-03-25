# Data Volume cloning with Smart-Cloning

## Introduction
Data Volumes (DV) can be created also by specifying a PVC as an input source. It will trigger a clone of the original PVC. See more details [here](datavolumes.md#pvc-source).

CDI implementation of cloning a PVC is done with host assisted cloning by streaming the data from the source PVC and write to the new PVC.

In order to improve the performance of the cloning process, a Smart-Cloning flow using snapshots is introduced.

## Smart-Cloning
CDI use the feature of creating a PVC from snapshot in order to clone PVCs more efficiently when a CSI plugin with snapshot capabilities is available.

The yaml structure and annotations of the DV are not changed.

### Create PVC from snapshot
Since Kubernetes v1.12, a feature enabling to create a PVC from a volume snapshot has been introduced. See more details [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-snapshot-and-restore-volume-from-snapshot-support)

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
  * Trigger a host-assisted clone

