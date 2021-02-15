# How to clone an image from a PV to a block PV
The purpose of this document is to show how to clone an image from an existing (file system or block) DV/PVC, to a target block PV.

## Prerequisites
- You have a Kubernetes cluster up and running with CDI installed, source DV/PVC, and at least one available block PersistentVolume to store the cloned disk image.
- When cloning from file system to block, content type must be kubevirt (default) in both source and target, and host-assisted clone is used.
- Feature-Gate 'BlockVolume' is enabled.


## Clone an image with DataVolume manifest

Create the following DataVolume manifest (clone-block-datavolume.yaml):

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: clone-block-datavolume
spec:
  source:
    pvc:
      namespace: "source-ns"
      name: "source-datavolume"
  pvc:
    volumeMode: Block
    accessModes:
      - ReadWriteOnce
    resources:
      requests:
        storage: 1Gi
```

Deploy the DataVolume manifest:

```bash
kubectl create -f clone-block-pv-datavolume.yaml
```

Two cloning pods, source and target, will be spawned and the image existed on the source block PV, will be copied to the target block PV.
