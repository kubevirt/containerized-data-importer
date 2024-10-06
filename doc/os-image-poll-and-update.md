# Automated OS image import, poll and update

CDI supports automating OS image import, poll and update, keeping OS images up-to-date according to the given `schedule`. On the first time a `DataImportCron` is scheduled, the controller will import the source image. On any following scheduled poll, if the source image digest (sha256) has updated, the controller will import it to a new [*source*](#dataimportcron-source-formats) in the `DataImportCron` namespace, and update the managed `DataSource` to point to the newly created source. A garbage collector (`garbageCollect: Outdated` enabled by default) is responsible to keep the last `importsToKeep` (3 by default) imported sources per `DataImportCron`, and delete older ones.

See design doc [here](https://github.com/kubevirt/community/blob/main/design-proposals/golden-image-delivery-and-update-pipeline.md)

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataImportCron
metadata:
  name: fedora-image-import-cron
  namespace: golden-images
spec:
  template:
    spec:
      source:
        registry:
          url: "docker://quay.io/kubevirt/fedora-cloud-registry-disk-demo:latest"
          pullMethod: node
          certConfigMap: some-certs
      storage:
        resources:
          requests:
            storage: 5Gi
        storageClassName: hostpath-provisioner
  schedule: "30 1 * * 1"
  garbageCollect: Outdated
  importsToKeep: 2
  managedDataSource: fedora
```

A `DataVolume` can use a `sourceRef` referring to a `DataSource`, instead of the `source`, so whenever created it will use the latest imported source similarly to specifying `dv.spec.source`. 

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: fedora-ref
  namespace: golden-images
spec:
  sourceRef:
      kind: DataSource
      name: fedora
  storage:
    resources:
      requests:
        storage: 5Gi
    storageClassName: hostpath-provisioner
```
## OpenShift ImageStreams

Using `pullMethod: node` we also support import from OpenShift `imageStream` instead of `url`:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataImportCron
metadata:
  name: rhel8-image-import-cron
  namespace: openshift-virtualization-os-images
spec:
  template:
    spec:
      source:
        registry:
          imageStream: rhel8-is
          pullMethod: node
      storage:
        resources:
          requests:
            storage: 5Gi
        storageClassName: hostpath-provisioner
  schedule: "0 0 * * 5"
  importsToKeep: 4
  managedDataSource: rhel8
```

Currently we assume the `ImageStream` is in the same namespace as the `DataImportCron`.

To create an `ImageStream` one can use for example:
* oc import-image rhel8-is -n openshift-virtualization-os-images --from=registry.redhat.io/rhel8/rhel-guest-image --scheduled --confirm
* oc set image-lookup rhel8-is -n openshift-virtualization-os-images 

Or on CRC:
* oc import-image cirros-is -n openshift-virtualization-os-images --from=kubevirt/cirros-container-disk-demo --scheduled --confirm
* oc set image-lookup cirros-is -n openshift-virtualization-os-images

More information on image streams is available [here](https://docs.openshift.com/container-platform/4.13/openshift_images/image-streams-manage.html) and [here](https://www.tutorialworks.com/openshift-imagestreams).

## DataImportCron source formats

* PersistentVolumeClaim
* VolumeSnapshot

DataImportCron was originally designed to only maintain PVC sources,  
However, for certain storage types, we know that snapshots sources scale better.  
Some details and examples can be found in [clone-from-volumesnapshot-source](./clone-from-volumesnapshot-source.md).

We keep this provisioner-specific information on the [StorageProfile](./storageprofile.md) object for each provisioner at the `dataImportCronSourceFormat` field (possible values are `snapshot`/`pvc`), which tells the DataImportCron which type of source is preferred for the provisioner.  

Some provisioners like ceph rbd are opted in automatically.  
To opt-in manually, one must edit the `StorageProfile`:
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: StorageProfile
metadata:
  ...
spec:
  dataImportCronSourceFormat: snapshot
```

To ensure smooth transition, existing DataImportCrons can be switchd to maintaining snapshots instead of PVCs by updating their corresponding storage profiles.

## DataImportCron storage class
Unless specified explicitly, similarly to PVCs, DataImportCrons will be provisioned using the default [virt](./datavolumes.md#default-virtualization-storage-class)/k8s storage class.  
In previous versions, an admin would have to actively delete the old sources upon change of the storage class  
(either explicitly by editing the DataImportCron or a cluster-wide change of the default storage class)  

Today, the controller performs this automatically;  
However, changing the storage class should be a conscious decision and in some cases (complex CI setups) it's advised to specify it explicitly
to avoid exercising a different storage class for golden images throughout installation.  
This flip flop could be costly and in some cases outright surprising to cluster admins.
