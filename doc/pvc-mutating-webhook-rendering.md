# PVC Mutating Webhook Rendering

## Introduction

PVC Mutating Webhook Rendering is an optional CDI feature, allowing users to get CDI PVC rendering functionality without using a `DataVolume`. Traditionally, when the CDI DV controller creates a PVC, it renders the PVC spec (`volumeMode`, `accessMode`, `storage`) according to the DV storage spec (or default) `storageClass`, CDI `StorageProfiles`, `CDIConfig` `filesystemOverhead` etc.

The PVC mutating webhook eliminates the need for DVs for `StorageProfile` based rendering, providing auto-completion of PVC missing spec fields, based on optimal values per `StorageClass`. The webhook intercepts only explicitly CDI-labeled PVCs, so it won't affect cluster stability if the CDI api server is down. For labeled PVCs, `objectSelectors` decide when to call out over HTTP to the webhook, so if the CDI api server is down the request and PVC creation will fail. Unlabeled PVC will not be affected at all.

CDI volume populators already cover almost all DV import/clone/upload functionality, but miss the PVC rendering functionality, so this feature complements CDI volume populators, as together they get most DV pros, but without its cons (e.g. limitations in backup and restore, disaster recovery).

## Configuration

To be fully compatible with any external tools that may already use CDI, this feature has to be enabled by the feature gate: `WebhookPvcRendering`. In the released `cdi-cr` it is enabled by default. To disable it, remove the feature gate from the `CDI` custom resource spec.config (see [cdi-config doc](./cdi-config.md)).

A Snippet below shows CDI resource with `WebhookPvcRendering` enabled.
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDI
[...]
spec:
  config:
    featureGates:
    - WebhookPvcRendering
[...]
```

## Usage

For any PVC you want to use `StorageProfile` mutating webhook rendering, label it with `cdi.kubevirt.io/applyStorageProfile: "true"`

If you want to use `volumeMode` preferred by CDI according to `StorageProfiles`, set it to `FromStorageProfile`. Otherwise if not explicitly set to `Block`, it will be `Filesystem` by k8s default.

## Examples

Blank PVC (missing `accessMode` and using CDI preferred `volumeMode`):

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: my-blank-pvc
  labels:
    cdi.kubevirt.io/applyStorageProfile: "true"
spec:
  storageClassName: rook-ceph-block
  volumeMode: FromStorageProfile
  resources:
    requests:
      storage: 1Mi
```

PVC imported using the import populator (missing `accessMode` and using the k8s default `Filesystem` `volumeMode`):

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name:  my-imported-pvc
  labels:
    cdi.kubevirt.io/applyStorageProfile: "true"
spec:
  dataSourceRef:
    apiGroup: cdi.kubevirt.io
    kind: VolumeImportSource
    name: my-import-source
  resources:
    requests:
      storage: 10Gi
```

PVC cloned using the clone populator (missing `accessModes`, and `storage` which is detected from the source PVC if bound):

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name:  my-cloned-pvc
  labels:
    cdi.kubevirt.io/applyStorageProfile: "true"
spec:
  dataSourceRef:
    apiGroup: cdi.kubevirt.io
    kind: VolumeCloneSource
    name: my-clone-source
```