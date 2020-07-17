# Local Storage Placement for VM Disks

This document describes a special handling of PVCs which have StoragaClass with `volumeBindingMode` set to `WaitForFirstConsumer`.  

## Introduction

Local Storage PVs are bound to a specific node. With the binding mode of `WaitForFirstConsumer`  
the binding and the provisioning is delayed until a Pod using the PVC is created. That way the Pod's scheduling constraints
are used to select or provision a PV. (TODO check how to reference external docu: see more on https://kubernetes.io/docs/concepts/storage/storage-classes/)

CDI uses worker Pod to import/upload/clone data to the PVC. Creating such a worker Pod will trigger binding of the PVC 
on the node where worker Pod is scheduled. Worker Pod might have different constraints than a kubevirt VM. When the VM is 
scheduled on a different node then PVC it becomes unusable. Problem might be even bigger when a VM has more than one PVC 
managed by CDI.

## Handling the WaitForFirstConsumer mode

The CDI has a special handling for storage with `WaitForFirstConsumer` mode.

DataVolume controller creates the PVC. When the underlying status of the PVC is Pending/WaitForFirstConsumer 
the CDI will not schedule any worker pods (import/clone/upload) associated with that PVC. The DataVolume controller sets 
the DV status to a new phase `WaitForFirstConsumer`. This allows the workload itself (ie. kubevirt) 
to detect a DV phase and handle the initial scheduling which causes the PVC to change to a Bound state.
After creating DV that specifies PVC 