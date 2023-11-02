# Efficient Data Volume cloning

### Introduction

Data Volumes cloning a PVC source support multiple forms of cloning. Based on the prerequisites listed [here](#Prerequisites), 
a cloning method will be utilized with varying degrees of efficiency. These forms are [CSI volume cloning](./csi-cloning.md), 
[smart cloning](./smart-clone.md), and [host-assisted cloning](./clone-datavolume.md). CSI volume cloning and smart cloning
employ CSI driver features in order to more efficiently clone a source PVC, but have certain limitations, 
while host-assited cloning provides a slower means with little limitation.

### Prerequisites
_The required prerequisites in order to trigger efficient cloning methods_
* **CSI Volume Cloning**:
    1) The csi driver backing the storage class of the PVC supports volume cloning, and corresponding StorageProfile has
       the cloneStrategy set to CSI Volume Cloning (see [here](./csi-cloning.md#Prerequisites) for more details)
    2) The source and target PVCs share the same Storage Class
    3) The source and target PVCs share the same Volume Mode
    4) The user creating the DataVolume has permission to create the `datavolumes/source` resource in the source namespace
    5) The source volume is not in use

* **Smart Cloning**:
    1) A Snapshot Class associated with the Storage Class exists
    2) The source and target PVCs share the same Storage Class
    3) The source and target PVCs share the same Volume Mode
    4) The user creating the DataVolume has permission to create the `datavolumes/source` resource in the source namespace
    5) The source volume is not in use

*Note: Data Volume Cloning can work together with namespace transfer and size expansion*  

### Fallback to Host-Assisted Cloning
Whenever the above cloning prerequisites are not met, we fallback to host-assisted cloning (`cloneType: copy`), which is the least efficient method of cloning. It uses a source pod and a target pod to copy data from the source volume to the target volume. In that case, the target PVC is annotated with the fallback reason and an event is emitted.

E.g. in the PVC:
```yaml
- apiVersion: v1
  kind: PersistentVolumeClaim
  metadata:
    annotations:
      cdi.kubevirt.io/cloneFallbackReason: The volume modes of source and target are incompatible
      cdi.kubevirt.io/clonePhase: Succeeded
      cdi.kubevirt.io/cloneType: copy
```

And in the events:
```
NAMESPACE   LAST SEEN   TYPE        REASON                      OBJECT                              MESSAGE
test-ns     0s          Warning     IncompatibleVolumeModes     persistentvolumeclaim/test-target   The volume modes of source and target are incompatible
```

### Additional Documentation
* DataVolumes: [datavolumes](./datavolumes.md)
* DataVolume Cloning: [clone-datavolumes](./clone-datavolume.md)
* CSI Volume Cloning: [csi-cloning](./csi-cloning.md)
* Cloning from VolumeSnapshot source: [clone-from-volumesnapshot-source](./clone-from-volumesnapshot-source.md)
* Smart Cloning: [smart-clone](./smart-clone.md)
* Namespace Transfer: [namespace-transfer](./namespace-transfer.md)
* StorageProfile (description of cloneStrategy): [storageprofile](./storageprofile.md)
