# Data Volume cloning from a VolumeSnapshot source

## Introduction
Current CDI PVC cloning methods offer [smart cloning](./smart-clone.md) which uses a 1:1 cloning method that creates a temporary snapshot of the source PVC for each intended clone.  
Some storage systems are designed to scale better with a 1:N method where a single snapshot of the source PVC is used to create multiple clones.  
One examples is ceph, where this would be about utilizing RBD layering.  
A much simpler example would be storage that can take around 5-10 minutes to snapshot a 200GB volume.  

*Note: Data Volume cloning from snapshot source can work together with namespace transfer and size expansion*  

### Prerequisites
  1) The source snapshot and target PVC share the same provisioner
  2) The user creating the DataVolume has permission to create the `datavolumes/source` resource in the source namespace
  3) Storage supports expansion (if the user attempts clone to larger target)

### Flow Description
- DataVolume is created with a Snapshot source
- Check if restore from said snapshot to desired target is possible (as described in Prerequisites):
- If possible:
    * Create the restore PVC
    * Set the claim reference of the PV to point to the new target PVC ([namespace-transfer](./namespace-transfer.md))
- If not possible:
    * Attempt [host-assisted cloning](./clone-datavolume.md) between 2 PVCs where CDI creates a temporary restore PVC (which will be cleaned up) from the snapshot to act as the source.

## Example
To kick off the process, we need a source volume snapshot.  
We create a simple Data Volume that will act as a source for the snapshot.
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: golden-snap-source
  namespace: golden-ns
spec:
  source:
      http:
         url: http://mirrors.nav.ro/fedora/linux/releases/33/Cloud/x86_64/images/Fedora-Cloud-Base-33-1.2.x86_64.qcow2
  storage:
    accessModes:
    - ReadWriteOnce
    resources:
      requests:
        storage: 8Gi
```
Next, we will create a VolumeSnapshot of the resulting PVC
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
Double check snapshot is ready to use (not a must, but nice to know)
```bash
$ kubectl get volumesnapshot golden-volumesnapshot 
NAME                    READYTOUSE   SOURCEPVC            SOURCESNAPSHOTCONTENT   RESTORESIZE   SNAPSHOTCLASS                            SNAPSHOTCONTENT                                    CREATIONTIME   AGE
golden-volumesnapshot   true ...
```
Now we will go ahead and create the target DataVolume that clones the above VolumeSnapshot
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: cloned-datavolume
  namespace: default
spec:
  source:
    snapshot:
      namespace: golden-ns
      name: golden-volumesnapshot
  storage:
    storageClassName: rook-ceph-block
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 9Gi
```
