# Namespace Transfer

## Summary

The Object Transfer API allows for logically moving PersistentVolumeClaims and DataVolumes between namespaces. It does this by mainpulating Kubernetes API resources and does not move any physical data on the volume.  This API is used internally by the CDI controller to facilitate efficient cross namespace cloning for DataVolumes.  It is also possible for cluster admins to use the Object Transfer API directly.  Given the following manifest:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: ObjectTransfer
metadata:
  name: t1
spec:
  source:
    kind: PersistentVolumeClaim
    namespace: source
    name: source-pvc
  target:
    namespace: destintation
    name: destination-pvc
```

The PersistentVolumeClaim `source-pvc` in the namespace `source` will be moved to the namespace `destination` with the given name `destination-pvc`.

Note that this is a cluster scoped resource.  A namespace scoped API is forthcoming.

## Transfer Operations

The following operations occur when the the ObjectTransfer above is executed:

1.  The PersistentVolume `source-pvc` is bound to (`source-pv`) is set to `Retain` if not already
2.  `source-pvc` is deleted
3.  `source-pv` claimRef is set to `destination\destination-pvc`
4.  `destination-pvc` is created with the same spec as `source-pvc` before it was deleted
