# Data Volume preallocation

## Introduction

In order to improve write performance, QEMU provides the ability to preallocate QCOW2 files, used
as backing for DataVolumes.

There are several methods for preallocation. CDI uses the best one available, depending on the
underlying file system and device type. It tries to use the OS's `fallocate` call if the filesystem
supports it and falls back to "full" preallocation for block devices. Preallocation does not depend
on the source of the DV, i.e. it can be used for import, upload or blank DVs.

See `qemu-img` [documentation](https://qemu.readthedocs.io/en/latest/system/images.html) to learn
more about preallocation. See also below for considerations regarding different datavolume types.

## Preallocation for a DataVolume

To preallocate space for a DataVolume, use `preallocation` field in DataVolume's spec:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: DataVolume
metadata:
  name: preallocated-datavolume
spec:
  source:
    ...
  pvc:
    ...
  preallocation: true
```

## Enabling preallocation globally

Preallocation can be also turned on for all DataVolumes with an entry in the `spec.config` of the `CDI` resource:

```bash
kubectl patch cdis.cdi.kubevirt.io cdi -p '{"spec": { "config": { "preallocation": true }}}' -o json --type merge
```

If not specified, the `preallocation` option defaults to false.

## Considerations

Preallocation can be used in the following cases:
- import (any source)
- upload
- cloning
- blank images
- blank block data images.

However, different DataVolume types use different preallocation methods:
- for blank block devices: the space is filled with zeros.
- for cloning volumes: handling sparse files is turned off, so the destination volume is filled in full, even if
  the source volume is not preallocated.
- blank images, upload and import volumes use qemu-img preallocation option, using `falloc` if available, and
  `full` otherwise.
