# Automated OS image (import), poll and update

CDI supports automating OS image import and keeping OS images up-to-date.

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
      pvc:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  schedule: "30 1 * * 1"
  garbageCollect: Outdated
  managedDataSource: fedora
```

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
      pvc:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 20Gi
  schedule: "30 1 * * 1"
  garbageCollect: Outdated
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