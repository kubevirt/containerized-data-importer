# Data Volume Annotations

## Introduction

CDI allows adding specific annotations to a DV or PVC to be passed to the Transfer Pods and control their behavior. Only specifically allowed annotations are passed, to prevent unexpected behavior due to missing testing coverage.

## Multus
Anntoations allows controlling which network will be used by the importer pod:
 * k8s.v1.cni.cncf.io/networks: networkname - pod will get both the default network from the cluster, and the secondary multus network.
 * v1.multus-cni.io/default-network: networkname - pod will get the multus network as its default network.

For example:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: dv-ann
  annotations:
      v1.multus-cni.io/default-network: bridge-network
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

Further reference on setting Multus secondary network can be found [here](https://kubevirt.io/2020/Multiple-Network-Attachments-with-bridge-CNI.html).

## AspenMesh / istio

 * sidecar.istio.io/inject: "false" - disables sidecar injection to the pod

For example:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: dv-am
  annotations:
      sidecar.istio.io/inject: "false"
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

To enable sidecar injection in namespace:

```bash
kubectl label namespace default istio-injection=enabled
kubectl get namespace default -L istio-injection
```