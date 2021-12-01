# Automated OS image import, poll and update

CDI supports automating OS image import, poll and update, keeping OS images up-to-date according to the given `schedule`. On the first time a `DataImportCron` is scheduled, the controller will import the source image. On any following scheduled poll, if the source image digest (sha256) has updated, the controller will import it to a new `PVC` in the `DataImportCron` namespace, and update the managed `DataSource` to point that `PVC`. A garbage collector (`garbageCollect: Outdated` enabled by default) is responsible to keep the last `importsToKeep` (3 by default) imported `PVCs` per `DataImportCron`, and delete older ones.

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

A `DataVolume` can use a `sourceRef` referring to a `DataSource`, instead of the `source`, so whenever created it will use the updated referred `PVC` similarly to a `source.PVC`. 

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

More information on image streams is available [here](https://docs.openshift.com/container-platform/4.8/openshift_images/image-streams-manage.html) and [here](https://www.tutorialworks.com/openshift-imagestreams).