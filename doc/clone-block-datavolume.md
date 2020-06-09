# How to clone an image from one block raw PV to another one
The purpose of this document is to show how to clone an image from an existing raw block PV, to another target block PV.

## Prerequisites
- You have a Kubernetes cluster up and running with CDI installed, block source PVC, and at least one available block PersistentVolume to store the cloned disk image.
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
