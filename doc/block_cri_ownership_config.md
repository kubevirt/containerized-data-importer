# device_ownership_from_security_context CRI configurable

## Introduction
Unlike volumes with fsGroup, devices have no official notion of deviceGroup/deviceUser that the CRI runtimes (or kubelet) would be able to use.  
This makes it problematic for our workloads to populate block devices, and has manifested itself in the form of [this](https://github.com/kubevirt/containerized-data-importer/issues/2433#issuecomment-1287277907) community issue.

## Solution
As explained in the source below, a solution that is seamless to end-users was chosen by the k8s community, without getting the device plugin vendors involved.  
The selected approach was to re-use `runAsUser` and `runAsGroup` for devices, with an opt-in config entry for the CRI (`device_ownership_from_security_context`) that ensures no existing deployment breaks.  
To use CDI, it is advised to opt-in.  
For containerd:
```toml
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    device_ownership_from_security_context = true
```
CRI-O:
```toml
[crio.runtime]
device_ownership_from_security_context = true
```

## Source
https://kubernetes.io/blog/2021/11/09/non-root-containers-and-devices/