# Data Volume cloning with CSI Volume Clone

## Introduction
CSI Volume Cloning uses the `Volume Cloning` feature of CSI drivers in order to perform a quick and efficient PVC clone.
The `Volume Cloning` functionality is not universal to all CSI drivers in kubernetes, so care must be taken when defining
and choosing the source PVC storage class. In order to enable CSI Cloning for the selected storage class, the 
storage profile of the respective DataVolume PVC will be checked for cloneStrategy (`cloneStrategy=csi-clone` see [Storage Profiles](./storageprofile.md)). 
It is the responsibility of a user to configure the storage profile accordingly.

See [volume-cloning](https://kubernetes-csi.github.io/docs/volume-cloning.html) for more information on discerning the capabilities of your csi driver.

### Prerequisites
  1) The csi driver backing the storage class of the PVC supports volume cloning, and corresponding StorageProfile has
     the cloneStrategy set to CSI Volume Cloning (see [here](./csi-cloning.md#Prerequisites) for more details)
  2) The source and target PVC share the same Storage Class (see [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class) for details)
  3) The source and target PVC share the same Volume Mode (see [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-mode) for details)
  4) The user creating the DataVolume has permission to create the `datavolumes/source` resource in the source namespace
  5) The source volume is not in use

### Flow Description
- DataVolume is created with a PVC source
- Check if CSI Volume Cloning is possible (as described in Prerequisites):
- If CSI Volume Cloning is possible:
    * Create the PVC
    * Set the claim reference of the PV to point to the new target PVC
- If CSI Volume Cloning is not possible:
    * Attempt Host Assisted Cloning [host-assisted cloning](./clone-datavolume.md)