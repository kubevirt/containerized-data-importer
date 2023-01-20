# Using external volume populators with DataVolumes

## Introduction
With the addition of the [volume populators feature](https://github.com/kubernetes/enhancements/tree/master/keps/sig-storage/1495-volume-populators) in Kubernetes, users can now specify custom sources of population for PVCs. In CDI, we want to allow users to benefit from these custom methods of population while maintaining the advantages of using DataVolumes, like spec rendering, progress reporting or storage-profile support. 

## Usage
Our DataVolume spec now has a new `DataSourceRef` field that allows users to specify external population sources. This mechanism works in the same way as it works with regular PVCs, so, for more information about its usage and characteristics, we recommend checking the official [Kubernetes documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#volume-populators-and-data-sources).

Also, see [Volume Populators Graduate to Beta](https://kubernetes.io/blog/2022/05/16/volume-populators-beta/) for more examples.

### Prerequisites
  1) Enable the `AnyVolumeDataSource` feature gate in kube-apiserver.
  2) Install a CRD for the specific populator.
  3) Install the populator controller itself.
  4) Create a DataVolume with a DataSourceRef field referencing the population source. Below, there's an example of a DataVolume that would be populated by a `SamplePopulator` populator.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: my-populator-dv
  annotations:
spec:
  storage:
    dataSourceRef:
        apiGroup: cdi.sample.populator
        kind: SamplePopulator
        name: sample-populator
    resources:
      requests:
        storage: 500Mi
```

Once the DataVolume is created with a populated DataSourceRef field, the DataVolume controller will create the appropriate PVC. Once the PVC is created, the controller waits for the PVC to be bound, and then checks the environment requirements to see if the population succeeded.
