# HOW-TO Maintain RPM lists for containers when adding new target platforms and architectures

## Overview

The file `rpms/BUILD.bazel` provides a list pinned rpms to be built in each category of CDI containers

These are maintained with bazeldnf with rpm repos and names of packages  specified in the following three files:

 1. `.bazelrc`
 2. `repo.yaml`
 3. `hack/build/rpm-deps.sh` 

Then, running `make rpm-deps` _should_ invoke bazeldnf through the bazel cdi builder container to populate the `rpm/BUILD.bazel` file 

Once rpm/BUILD.bazel is populated, it is used and re-used, pinning the rpms to be built into run & test container images to specific versions. 
This is why it is checked in to github per CDI release process. 

## Prerequisites and Caveats

`bazeldnf` is not provided in the CDI build container; this method of generating lists of pinned rpms uses `bazeldnf` on the host.  

Since `make rpm-defs` only generates text files not executable images, the architecture of the host you run it on doesn't actually matter.

`bazeldnf` is available as a binary release from https://github.com/rmohr/bazeldnf/releases . 

Note that version 0.5.9-rc2 https://github.com/rmohr/bazeldnf/releases/tag/v0.5.9-rc2 is the first binary release for s390x. 

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
