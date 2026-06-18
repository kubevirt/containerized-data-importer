# PVC Mutating Webhook Rendering

## Introduction

PVC Mutating Webhook Rendering is an optional CDI feature, allowing users to get CDI PVC rendering functionality without using a `DataVolume`. Traditionally, when the CDI DV controller creates a PVC, it renders the PVC spec (`volumeMode`, `accessMode`, `storage`) according to the DV storage spec (or default) `storageClass`, CDI `StorageProfiles`, `CDIConfig` `filesystemOverhead` etc.

The PVC mutating webhook eliminates the need for DVs for `StorageProfile` based rendering, providing auto-completion of PVC missing spec fields, based on optimal values per `StorageClass`. The webhook intercepts only explicitly CDI-labeled PVCs, so it won't affect cluster stability if the CDI api server is down. For labeled PVCs, `objectSelectors` decide when to call out over HTTP to the webhook, so if the CDI api server is down the request and PVC creation will fail. Unlabeled PVC will not be affected at all.

CDI volume populators already cover almost all DV import/clone/upload functionality, but miss the PVC rendering functionality, so this feature complements CDI volume populators, as together they get most DV pros, but without its cons (e.g. limitations in backup and restore, disaster recovery).

## Configuration

PVC Mutating Webhook Rendering is **enabled by default** in CDI. The webhook is always deployed and requires no explicit configuration to activate.

To opt in to webhook auto-completion on a PVC, apply the label `cdi.kubevirt.io/applyStorageProfile: "true"` (see [Usage](#usage) below).

### Disabling the webhook

In case of emergency (e.g. webhook misbehavior), you can disable it by setting `disableWebhookPvcRendering: true` in the CDI config spec:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDI
[...]
spec:
  config:
    disableWebhookPvcRendering: true
[...]
```

When disabled, the `MutatingWebhookConfiguration` is removed and the DataVolume controller falls back to rendering PVC fields itself.

To re-enable, remove the field or set it to `false`.

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