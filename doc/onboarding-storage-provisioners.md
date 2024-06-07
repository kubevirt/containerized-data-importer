# Onboarding a storage provider to CDI

## Introduction

As explained extensively in the [storage profile](./storageprofile.md) documentation, CDI keeps internal knowledge  
about a wide range of storage providers in order to provide defaults & hints as they are needed.  
This doc is about "educating" CDI about a new storage provider.

## StorageProfile.Spec fields
In order to onboard a storage provider, one should be able to pick the correct values for the storage profile parameters.  
Detailed explanations and tips can be found in the storage profile's parameters [section](./storageprofile.md#parameters).

For example, for ceph rbd, CDI knows that:
- These are the access/volume modes (in this preference order) supported
https://github.com/kubevirt/containerized-data-importer/blob/c6089cbcb01ab58e75b25be1770ce0f3f4625014/pkg/storagecapabilities/storagecapabilities.go#L255-L261
- The preferred clone strategy is csi-clone
https://github.com/kubevirt/containerized-data-importer/blob/c6089cbcb01ab58e75b25be1770ce0f3f4625014/pkg/storagecapabilities/storagecapabilities.go#L115-L124
- Ceph rbd scales better when golden sources are stored as snapshots (more info in [clone-from-volumesnapshot-source](./clone-from-volumesnapshot-source.md))
https://github.com/kubevirt/containerized-data-importer/blob/c6089cbcb01ab58e75b25be1770ce0f3f4625014/pkg/storagecapabilities/storagecapabilities.go#L109-L112

## Default Virtualization Storage Class
A default virtualization storage class is defined as a storage class preferrable for VM workloads (certain combination of storage class parameters that benefit VMs) and is annotated with `storageclass.kubevirt.io/is-default-virt-class` set to `"true"`.  
For a DataVolume request that does not explicitly specify a storage class name, such a storage class takes precedence over the k8s default storage class.  
Advertising such a storage class via documentation makes sense in [certain cases](https://bugzilla.redhat.com/show_bug.cgi?id=2109455).

## Checkup framework - kubevirt storage checkup
In order to validate the changes and choices that were made, one can run the https://github.com/kiagnose/kubevirt-storage-checkup