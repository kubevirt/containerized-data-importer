# Debugging CDI

## Introduction

There are various instrumentations available for developers debugging CDI. We will keep expanding this document to make our life easier.

## DataVolume annotation to retain the transfer pods after completion

Adding the annotation `cdi.kubevirt.io/storage.pod.retainAfterCompletion: "true"` will cause CDI transfer pods (importer, uploader, cloner) to be retained after a successful or failed completion. This makes debugging and testing easier, as developers can get the pod state and logs after completion. The pods will be deleted when their dv/pvc is deleted, otherwise the user is responsible for deleting them.

For example:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: dv-pod-retain
  annotations:
      cdi.kubevirt.io/storage.pod.retainAfterCompletion: "true"
spec:
  source:
      http:
         url: "http://mirrors.nav.ro/fedora/linux/releases/33/Cloud/x86_64/images/Fedora-Cloud-Base-33-1.2.x86_64.qcow2"
  pvc:
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
```