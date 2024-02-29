# HOW-TO Maintain RPM lists for containers when adding new target platforms and architectures

## Overview

The file `rpms/BUILD.bazel` provides a list pinned rpms to be built in each category of CDI containers

These are maintained with bazeldnf with rpm repos and names of packages  specified in the following three files:

 1. `.bazelrc`
 2. `repo.yaml`
 3. `hack/build/rpm-deps.sh` 

Then, running `make rpm-deps` invokes bazeldnf installed in the bazel cdi builder container volume to populate the `rpm/BUILD.bazel` file 

## Prerequisites and Caveats

`bazeldnf` is not provided in the persistent CDI build container; it is built and installed from a pinned source tarball in the `make rpm-defs` step as per https://github.com/kubevirt/containerized-data-importer/blob/main/WORKSPACE#L83-L95 -- so if you _exec_ into the running container, you won't find the executable of bazeldnf in your bash path; rather it is only made available to bazel build as a package, e.g.:

```
[containerized-data-importer]$ ./hack/build/bazel-docker.sh bash
CDI_CRI: podman, CDI_CONTAINER_BUILDCMD: buildah
Making sure output directory exists...
go version go1.21.5 linux/s390x
go version go1.21.5 linux/s390x
Starting rsyncd

Rsyncing /home/cfillekes/projects/containerized-data-importer to container
8def2759249580cd4b7880fa2a79c554611a1033fe2d3f9bcc15aa3a79008c89
Starting bazel server
go version go1.21.5 linux/s390x
go version go1.21.5 linux/s390x
[root@m1325001 containerized-data-importer]# which bazeldnf
/usr/bin/which: no bazeldnf in (/root/.local/bin:/root/bin:/gimme/.gimme/versions/go1.21.5.linux.s390x/bin:/root/go/bin:/go/bin:/opt/gradle/gradle-6.6/bin:/gimme/.gimme/versions/go1.21.5.linux.s390x/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin)
[root@m1325001 containerized-data-importer]# find / -name bazeldnf
find: ‘/sys/fs/pstore’: Permission denied
find: ‘/sys/fs/bpf’: Permission denied
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/execroot/__main__/bazel-out/host/bin/external/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/execroot/__main__/bazel-out/host/bin/external/bazeldnf/pkg/api/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/execroot/__main__/bazel-out/s390x-fastbuild/bin/bazeldnf.bash.runfiles/__main__/external/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/execroot/__main__/bazel-out/s390x-fastbuild/bin/bazeldnf.bash.runfiles/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/execroot/__main__/external/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/external/bazeldnf
/root/.cache/bazel/_bazel_root/e03bf9038fc5089c0dbc8615812d9838/external/bazeldnf/pkg/api/bazeldnf
```


## Configuring the platform, build, run and test target to maintain pinned rpm lists

### add platform targets to `.bazelrc`

This defines which platforms we are going to build for, and how.  For example, adding `aarch64` and `s390x` native and cross-built platform targets:

```
build:aarch64 --platforms=@io_bazel_rules_go//go/toolchain:linux_arm64_cgo --incompatible_use_cc_configure_from_rules_cc
run:aarch64 --platforms=@io_bazel_rules_go//go/toolchain:linux_arm64_cgo --incompatible_use_cc_configure_from_rules_cc
test:aarch64 --platforms=@io_bazel_rules_go//go/toolchain:linux_arm64_cgo --host_javabase=@local_jdk//:jdk

build:crossbuild-aarch64 --incompatible_enable_cc_toolchain_resolution --platforms=//bazel/platforms:aarch64-none-linux-gnu --platforms=@io_bazel_rules_go//go/toolchain:lin
ux_arm64_cgo
run:crossbuild-aarch64  --incompatible_enable_cc_toolchain_resolution --platforms=//bazel/platforms:aarch64-none-linux-gnu --platforms=@io_bazel_rules_go//go/toolchain:linu
x_arm64_cgo
test:crossbuild-aarch64 --platforms=@io_bazel_rules_go//go/toolchain:linux_arm64_cgo --host_javabase=@local_jdk//:jdk
``` 

NB in future, when we add native and cross-compiled targets for ppc64le and s390x, they are similarly defined in `.bazelrc`

### add targets to `repo.yaml`

`repo.yaml` specifies the URLs of the repositories in which the packages to be installed via rpm are to be found, e.g.:

```
- arch: aarch64
  baseurl: http://mirror.stream.centos.org/9-stream/BaseOS/aarch64/os/
  name: centos/stream9-baseos-aarch64
  gpgkey: https://www.stream.centos.org/keys/RPM-GPG-KEY-CentOS-Official
```

### add targets to `hack/build/rpm-deps.sh`

`hack/build/rpm-deps.sh` provides the list of dnf packages from which the rpm meta data is to be extracted for each class of container targets, and the flags to run bazeldnf with from inside the bazel build container. 

```
centos_base="
  ca-certificates
  crypto-policies
  acl
  curl
  vim-minimal
  util-linux-core
"
```
where `centos_base` is referred to in the `bazel run //:bazeldnf` job like so:

```
bazel run \
    --config=aarch64 \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_aarch64 --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra
```

For further documentation on how `bazeldnf` works, please consult https://github.com/rmohr/bazeldnf
