# DataVolume Claim Adoption

## Summary

Historically, it has been an error to create a `DataVolume` with the same name as an existing `PersistentVolumeClaim`.

```bash
✗ k get dv dv1
Error from server (NotFound): datavolumes.cdi.kubevirt.io "dv1" not found

k get pvc dv1
NAME   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS      AGE
dv1    Bound    pvc-de8b1186-fdd1-4372-8135-9c904ecf73bc   7Gi        RWX            rook-ceph-block   9m7s

✗ k create -f dv.yaml
Error from server: error when creating "dv.yaml": admission webhook "datavolume-validate.cdi.kubevirt.io" denied the request:  Destination PVC default/dv1 already exists
```

However in certain cases, this behavior is not desirable.  It is often problematic when restoring from a backup.  In that case, the restore process will typically create a new `PersistentVolumeClaim` from a `VolumeSnapshot` and then fail when the corresponding `DataVolume` is created later.  In order to have a proper running application it is important to ensure all resources and their ownership relations are restored correctly.

We now offer two options for `DataVolumes` to "adopt" existing `PersistentVolumeClaims`.  When a `DataVolume` adopts an existing claim, the DataVolume is instantly marked `Succeeded` and an `OwnershipReference` is added to the `PersistentVolumeClaim`.

## cdi.kubevirt.io/allowClaimAdoption Annotation

Add this annotation to your DataVolume when you want to support claim adoption on a granular level.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: dv1
  annotations:
    cdi.kubevirt.io/allowClaimAdoption: "true"
    ...
```

## DataVolumeClaimAdoption Feature Gate

Enable this feature gate to have claim adoption applied automatically for all DataVolumes by default.  However, the `cdi.kubevirt.io/allowClaimAdoption` annotation has precedence over the feature gate.  Setting the value to `false` will disable claim adoption even if the feature gate is set.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDI
metadata:
  name: cdi
...
spec:
  config:
    featureGates:
    - DataVolumeClaimAdoption
    ...
```
