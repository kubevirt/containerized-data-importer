# Storage Profiles

## Introduction

Storage Profile is the resource that serves the information about recommended parameters for PVC - right now the accessMode and volumeMode.
This can be used by CDI controllers when creating a PVC for DV. That way the DataVolume can be simplified and if the properties are missing,
defaults can be applied from the StorageProfile.

CDI provides a collection of Storage Profiles with default recommended values for some well known backends. 
If the storage provisioner defined in storage class does not have defaults configured in CDI the resulting StorageProfile 
has empty `claimPropertySets`.

CDI automatically creates the StorageProfile objects - one StorageProfile per 
one StorageClass that exists in the cluster. Below StorageProfile for hostpath-provisioner as an example.

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: StorageProfile
metadata: 
  name: hostpath-provisioner
spec: 
  claimPropertySets: 
  - accessModes:
    - ReadWriteOnce
    volumeMode: 
      Filesystem
status:
    storageClass: hostpath-provisioner
    provisioner: kubevirt.io/hostpath-provisioner
    claimPropertySets: 
    - accessModes: 
      - ReadWriteOnce
      volumeMode: Filesystem
```

Values for accessModes and volumeMode are exactly the same as for PVC: `accessModes` is a list of `[ReadWriteMany|ReadWriteOnce|ReadOnlyMany]`
and `volumeMode` is a single value `Filesystem` or `Block`. Multiple claim property sets can be specified (`claimPropertySets` is a list) 
but currently CDI will ignore all but the first one.

## Handling the DV with defaults from Storage Profiles 

The example uses the `hpp` (`kubevirt.io/hostpath-provisioner`) as the storage provisioner.
For brevity some fields managed by kubernetes, like managedFields or creationTimestamp, were removed from output. 

1. Given the `hostpath-provisioner` StorageClass, CDI creates a `hostpath-provisioner` StorageProfile

`kubectl get sc hostpath-provisioner -o yaml`
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
  name: hostpath-provisioner
provisioner: kubevirt.io/hostpath-provisioner
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```

`k get storageprofile hostpath-provisioner -o yaml`
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: StorageProfile
metadata:
  labels:
    app: containerized-data-importer
    cdi.kubevirt.io: ""
  name: hostpath-provisioner
  ownerReferences: 
    ...
spec: {}
status:
  claimPropertySets:
  - accessModes:
    - ReadWriteOnce
    volumeMode: Filesystem
  provisioner: kubevirt.io/hostpath-provisioner
  storageClass: hostpath-provisioner
```

2. Now the user can create a new DV without specifying accessModes or volumeMode for the PVC.

`cat dv.yaml`
```yaml
apiVersion: cdi.kubevirt.io/v1alpha1
kind: DataVolume
metadata:
  name: blank-dv
spec:
  pvc:
    resources:
      requests:
        storage: 1Gi
    storageClassName: hostpath-provisioner
  source:
    blank: {}
```
`kubectl create -f dv.yaml`

Notice missing accessModes and volumeMode from `*.spec.pvc` on DV.
```yaml
accessModes:
- ReadWriteOnce
volumeMode: Filesystem
```

3. As s result the following pvc is created.
   
`kubectl get pvc blank-dv -o yaml`

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    cdi.kubevirt.io/storage.condition.running.reason: Completed
    ...
  creationTimestamp: "2021-03-04T14:19:33Z"
  finalizers:
  - kubernetes.io/pvc-protection
  labels:
    app: containerized-data-importer
  name: blank-dv
  namespace: default
  ownerReferences: 
    ...
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
  storageClassName: hostpath-provisioner
  volumeMode: Filesystem
  volumeName: pvc-a1c62357-dbfd-4909-aed7-19a88fa1e643
status:
  accessModes:
  - ReadWriteOnce
  capacity:
    storage: 2Gi
  phase: Bound
```
Notice how accessModes is ReadWriteOnce and volumeMode is Filesystem, exactly as configured in the Storageprofile.

## Empty Storage Profile

Not all provisioners have recommended parameters provided by CDI. In a case where no recommendation is available, CDI creates an empty Storage Profile.

`kubectl get storageprofile some-unknown-provisioner -o yaml`
```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: StorageProfile
metadata:
  name: some-unknown-provisioner-class
    ...
spec: {}
status:
  provisioner: some-unknown-provisioner
  storageClass: some-unknown-provisioner-class
```
There are no recommended parameters on StorageProfile so it is not possible to create a PVC for a DV without accessModes configured.   

`kubectl describe dv blank-dv`
```yaml
Name:         blank-dv
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  cdi.kubevirt.io/v1beta1
Kind:         DataVolume
Metadata:
 ...
Spec:
  Pvc:
    Resources:
      Requests:
        Storage:         1Gi
    Storage Class Name:  local
  Source:
    Blank:
Events:
  Type     Reason            Age                From                   Message
  ----     ------            ----               ----                   -------
  Warning  ErrClaimNotValid  1s (x12 over 18s)  datavolume-controller  DataVolume spec is missing accessMode and cannot get access mode from StorageProfile local
```

Notice the event on the DV.

## User defined Storage Profile

User with access rights to edit StorageProfile can configure recommended parameters. Edit spec section of StorageProfile by adding claimPropertySets with accessModes and volumeMode.
Shortly, all provided parameters should be visible in the status section. User defined parameter has higher priority and overrides the one provided by CDI. 

## Priorities

1. Parameter defined on DV
2. User provided parameters - defined on StorageProfile spec section.
3. Parameters provided by CDI.
4. Empty or kubernetes defaults (if available).




