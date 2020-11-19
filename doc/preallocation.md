# Data Volume preallocation

## Introduction

In order to improve write performance, QEMU provides the ability to preallocate QCOW2 files, used
as backing for DataVolumes.

There are several methods for preallocation. CDI uses the best one available, depending on the
underlying file system and device type. It tries to use the OS's `fallocate` call if the filesystem
supports it and falls back to "full" preallocation for block devices. Preallocation does not depend
on the source of the DV, i.e. it can be used for import, upload or blank DVs.

See `qemu-img` [documentation](https://qemu.readthedocs.io/en/latest/system/images.html) to learn
more about preallocation.

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

Preallocation can be also turned on for all DataVolumes with the following CDIConfig entry:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDIConfig
metadata:
  name: config
spec:
  preallocation:
    global: true
```

If not specified, the `preallocation` option defaults to false.

## Preallocation for StorageClass

It is also possible to define preallocation for all Data Volumes created with a StorageClass.
`CDIConfig` is used for that, similar to `filesystemOverhead`:

```yaml
apiVersion: cdi.kubevirt.io/v1beta1
kind: CDIConfig
metadata:
  name: config
spec:
  preallocation:
    # global not specified, defaults to false
    storageClass:
      sc-with-preallocation: true
      another-storage-class: false
```

## Order of precedence

In order to decide what preallocation setting to use, CDI first checks DataVolume spec. If the
option is present, its value is used. If not, CDI checks CDIConfig setting for the relevant
StorageClass. Again, if it is defined, it is used. If not, the global setting from CDIConfig
is used. Finally, if none of this options is specified, preallocation defaults to false.

In other words, `preallocation` option in DataVolume overrides StorageClass setting, and
`preallocation` option for StorageClass overrides global setting in CDIConfig.