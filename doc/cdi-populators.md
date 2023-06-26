# CDI Volume Populators

## Recommended knowledge

What are [populators](https://kubernetes.io/blog/2022/05/16/volume-populators-beta/)

## Introduction
CDI volume populators are CDIs implementation of populating PVCs by importing/uploading/cloning data utilizing the new `dataSourceRef` field. New controllers and custom resources for each population method were introduced.

**Values of the new API?**
* Native synchronization with Kubernetes - this is kubernetes way of populating PVCs. Once PVC is bound we know it is populated (So far PVC was bound the moment the datavolume created it and the population progress was monitored via the datavolume)
* Use PVCs directly and get them populated without datavolumes mitigation.
* Can use one population definition for multiple PVCs - create 1 CR defining population source and use it for any PVC.
* Better compatibility with existing backup solutions. Using PVC alone should solve all backup issues. Using datavolumes with populators solves most, for example Metro DR and [Gitops](https://www.redhat.com/en/topics/devops/what-is-gitops#:~:text=GitOps%20uses%20Git%20repositories%20as,set%20for%20the%20application%20framework.) - datavolume manifest will be applied, the datavolume will create the PVC that will bind immediately to the PV waiting for it.
* Better compatibility with integration with [Kubevirt](https://github.com/kubevirt/kubevirt) and existing backup solutions, using VMs with PVCs using populators and VMs with datavolumetemplates.
* Integration with [Kubevirt](https://github.com/kubevirt/kubevirt) with WFFC storage class is simpler not requiring [doppelganger pod](https://github.com/kubevirt/kubevirt/blob/main/docs/localstorage-disks.md#the-problem) for the start of the VM.


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
Example for PVC which will be handled by the import populator:
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
For upload need to follow the same guidelines as describe in the [upload doc](upload.md) but instead of creating a data volume you can create VolumeUploadSource CR and PVC similar to the import examples in this doc.

#### Clone
Same for clone very similar API:
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
If the used storage class is CSI storage then the datavolume population will occur via the CDI populators with the end result as always has been of a populated PVC. You will be able to notice that the created PVC will stay pending until the population process completes.
For more information of using datavolumes for population check the [datavolume doc](datavolumes.md)

> NOTE: Datavolumes and the PVCs they create will be marked with "usePopulator" Annotation to indicate the population is done via CDI populators
