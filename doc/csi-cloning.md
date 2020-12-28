# Data Volume cloning with CSI Volume Clone

## Introduction
CSI Volume Cloning uses the `Volume Cloning` feature of CSI drivers in order to perform a quick and efficient PVC clone. The `Volume Cloning` functionality is not universal to all CSI drivers in kubernetes, so care must be taken when defining and chosing the source PVC storage class. In order to verify whether the csi driver backing the PVC supports volume cloning, the storage class of the respective DataVolume PVC will be checked for the annotation `cdi.kubevirt.io/CSICloneVolumeCapable = true`. It is the responsibility of the cluster admin to annotate storage classes accordingly. See [volume-cloning](https://kubernetes-csi.github.io/docs/volume-cloning.html) for more information on discerning the capabilities of your csi driver.

### Prerequisites
  1) The csi driver backing the storage class of the PVC supports volume cloning, and has been annotated accordingly by a cluster admin
  2) The source and target PVC share the same Storage Class (see [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#class) for details)
  3) The source and target PVC share the same Volume Mode (see [here](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-mode) for details)
  4) The user creating the DataVolume has permission to create the `datavolumes/source` resource in the source namespace

### Flow Description
- DataVolume is created with a PVC source
- Check if CSI Volume Cloning is possible:
  * The source and target PVCs must be in the same Storage Class
  * The source and target PVC share the same Volume Mode
  * The Storage Class is annotated `cdi.kubevirt.io/CSICloneVolumeCapable = true`
- If CSI Volume Cloning is possible:
  * Create a temporary "cloning PVC" in source namespace with dataSource set to the source PVC
  * Create a PVC in the target namespace with volumeName set to the PV of the "cloning PVC"
  * Set the claim reference of the PV to point to the new target PVC
  * Delete the "cloning PVC"
- If CSI Volume Cloning is not possible:
  * Attempt Smart Cloning [smart-clone](./smart-clone.md)