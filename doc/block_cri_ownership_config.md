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

## k3s and RKE2 (managed containerd configuration)
Distributions such as k3s and RKE2 generate the containerd `config.toml` on every service start, so editing the file directly does not persist: the change is overwritten on the next restart.

On k3s, prefer the built-in flag, which sets `device_ownership_from_security_context = true` in the generated config (available in recent releases, for example v1.32+):
```bash
# CLI flag on server/agent
k3s server --nonroot-devices

# or in /etc/rancher/k3s/config.yaml
nonroot-devices: true
```
Restart the k3s service on each node afterwards, then recreate any affected CDI pods (already-running pods keep their original device ownership).

If your version does not have the flag, the supported customization path is a containerd config template (`config-v3.toml.tmpl` for containerd v2, `config.toml.tmpl` for containerd v1, in the same directory as the generated `config.toml`). Note one pitfall: TOML does not allow redefining a table, so appending a `[plugins."io.containerd.cri.v1.runtime"]` section after `{{ template "base" . }}` fails with `table io.containerd.cri.v1.runtime already exists` and containerd will not start, leaving the node NotReady. To modify a value that the base template already sets, the template must be a full copy of the generated `config.toml` with the value edited in place. A full-copy template does not pick up future base config changes, so review it after upgrades.

RKE2 uses the same template mechanism and the same pitfall applies; check whether your RKE2 version offers an equivalent flag before falling back to a template.

Verified on k3s v1.34.6 (containerd 2.x) with rook-ceph RBD block-mode PVCs: see [kubevirt/kubevirt#14335](https://github.com/kubevirt/kubevirt/issues/14335).

## Source
https://kubernetes.io/blog/2021/11/09/non-root-containers-and-devices/
