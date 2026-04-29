# device_ownership_from_security_context CRI configurable

## Symptoms
When `device_ownership_from_security_context` is not enabled in the CRI runtime configuration, CDI pods that operate on block-mode PersistentVolumes will fail with permission errors. Common error logs:

- **Upload to block PVC** — the upload server fails with:
  ```
  Saving stream failed: Unable to transfer source data to target file: error determining if block device exists: exit status 1, blockdev: cannot open /dev/cdi-block-volume: Permission denied
  ```
- **Import to block PVC** — the importer pod logs `blockdev: cannot open /dev/cdi-block-volume: Permission denied`, then reports `Target size -1` and misidentifies the block volume as a filesystem volume:
  ```
  E0929 02:13:54.319027       1 data-processor.go:383] exit status 1, blockdev: cannot open /dev/cdi-block-volume: Permission denied
  I0929 02:13:54.319110       1 data-processor.go:404] Target size -1.
  ```

## Introduction
Unlike volumes with fsGroup, devices have no official notion of deviceGroup/deviceUser that the CRI runtimes (or kubelet) would be able to use.  
This makes it problematic for our workloads to populate block devices, and has manifested itself in the form of [this](https://github.com/kubevirt/containerized-data-importer/issues/2433#issuecomment-1287277907) community issue.

## Solution
As explained in the source below, a solution that is seamless to end-users was chosen by the k8s community, without getting the device plugin vendors involved.  
The selected approach was to re-use `runAsUser` and `runAsGroup` for devices, with an opt-in config entry for the CRI (`device_ownership_from_security_context`) that ensures no existing deployment breaks.  
To use CDI, it is advised to opt-in.  
For Containerd v1:
```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = true
```
For Containerd v2:
```toml
[plugins]
  [plugins."io.containerd.cri.v1.runtime"]
    device_ownership_from_security_context = true
```
CRI-O:
```toml
[crio.runtime]
device_ownership_from_security_context = true
```

## Source
https://kubernetes.io/blog/2021/11/09/non-root-containers-and-devices/
