# CDI Volume Populators

## Recommended knowledge

What are [populators](https://kubernetes.io/blog/2022/05/16/volume-populators-beta/)

## Introduction
CDI volume populators are CDI's implementation of importing/uploading/cloning data to PVCs using the new `dataSourceRef` field. New controllers and custom resources for each population method were introduced.

**Benefits of the new API**
* Native synchronization with Kubernetes - this is Kubernetes way of populating PVCs. Once PVC is bound we know it is populated (Before introducing populators, the PVC was bound the moment the datavolume created it, and the population progress was monitored via the datavolume)
* Use PVCs directly and have them populated without the need for datavolumes.
* Can use one population definition for multiple PVCs - create 1 CR defining population source and use it for any PVC.
* Better compatibility with existing backup and restore solutions:
    * Standalone PVC should be compatible with all solutions.
    * Datavolumes using the populators should be compatible with most solutions, for example Metro DR and [Gitops](https://www.redhat.com/en/topics/devops/what-is-gitops#:~:text=GitOps%20uses%20Git%20repositories%20as,set%20for%20the%20application%20framework.) - when restoring datavolume manifest will be applied, the datavolume will create the PVC that will bind immediately to the PV waiting for it.
    * [Kubevirt](https://github.com/kubevirt/kubevirt) VMs with PVCs/datavolumetemplate should also be compatible with most solutions.
* Integration with [Kubevirt](https://github.com/kubevirt/kubevirt) with WaitForFirstConsumer(WFFC) storage class is simpler and does not require a [doppelganger pod](https://github.com/kubevirt/kubevirt/blob/main/docs/localstorage-disks.md#the-problem) for the start of the VM.


## Using the populators

We introduced new controllers for each population method. Each controller supports a matching CRD which provides the needed information for the population.
Example for an instance of VolumeImportSource CRD:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: VolumeImportSource
metadata:
  name: my-import-source
spec:
  source:
      http:
         url: "https://download.cirros-cloud.net/0.4.0/cirros-0.4.0-x86_64-disk.img"
```

### Using populators with PVCs
User can create a CR and PVCs specifying the CR in the `DataSourceRef` field and those will be handled by the matching populator controller.

#### Import
Example for PVC which use the VolumeImportSource above that will be handled by the import populator:
```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name:  my-pvc
spec:
  dataSourceRef:
    apiGroup: cdi.kubevirt.io
    kind: VolumeImportSource
    name: my-import-source
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```

Above PVC will trigger the reconcile of the import populator controller.
The controller will create a matching temporary PVC with the appropriate annotations, which will get bound and populated.
Once the temporary PVC population is done, the PV will be rebound to the original PVC completing the population process.

#### Upload
Example of VolumeUploadSource and a PVC that will be handled by the upload populator:
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: VolumeUploadSource
metadata:
  name: my-upload-source
spec:
   contentType: kubevirt
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name:  my-pvc
spec:
  dataSourceRef:
    apiGroup: cdi.kubevirt.io
    kind: VolumeUploadSource
    name: my-upload-source
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
```
After creating the VolumeUploadSource and PVC, you can start the upload to the pvc as describe in the [upload doc](upload.md).

#### Clone
Example of VolumeCloneSource and a PVC that will be handled by the clone populator:
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: VolumeCloneSource
metadata:
  name: my-clone-source
spec:
  source:
    kind: PersistentVolumeClaim
    name: my-pvc-source
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name:  my-cloned-pvc
spec:
  dataSourceRef:
    apiGroup: cdi.kubevirt.io
    kind: VolumeCloneSource
    name: my-clone-source
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi

```

### Using populators with DataVolumes

The integration of datavolumes and CDI populators is seamless. You can create the datavolumes the same way you always have.
If the DataVolume targets a storage class that uses a CSI provisioner CDI will automatically use the new populators method.
The behavior will be the same as always with the following key differences:
* The created PVC will become bound only once the population process completes.
* In case of WFFC binding mode the datavolume status will be PendingPopulation instead of WaitForFirstConsumer.

> NOTE: Datavolumes and the PVCs they create will be marked with "usePopulator" Annotation to indicate the population is done via CDI populators

For more information of using datavolumes for population check the [datavolume doc](datavolumes.md)

### Fallback to legacy population

In some cases, CDI will fall back to legacy population methods, and thus skip using volume populators when:
* Storage provisioner is non-CSI
* Annotation `cdi.kubevirt.io/storage.usePopulator` set to `"false"`
