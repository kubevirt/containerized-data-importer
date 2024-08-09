#!/usr/bin/env bash

set -ex

source hack/build/common.sh
source hack/build/config.sh

bazeldnf_repos="--repofile repo.yaml"
if [ "${CUSTOM_REPO}" ]; then
    bazeldnf_repos="--repofile ${CUSTOM_REPO} ${bazeldnf_repos}"
fi

# Packages that we want to be included in all container images.
#
# Further down we define per-image package lists, which are just like
# this one are split into two: one for the packages that we actually
# want to have in the image, and one for (indirect) dependencies that
# have more than one way of being resolved. Listing the latter
# explicitly ensures that bazeldnf always reaches the same solution
# and thus keeps things reproducible
centos_base="
  ca-certificates
  crypto-policies
  acl
  curl
  vim-minimal
  util-linux-core
"
centos_extra="
  coreutils-single
  glibc-minimal-langpack
  libcurl-minimal
  tar
"

# get latest repo data from repo.yaml
bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- fetch ${bazeldnf_repos}

cdi_importer="
libnbd
libstdc++
nbdkit-server
nbdkit-basic-filters
nbdkit-curl-plugin
nbdkit-xz-filter
nbdkit-gzip-filter
qemu-img
python3-pycurl
python3-six
"

cdi_importer_extra_x86_64="
nbdkit-vddk-plugin
sqlite-libs
ovirt-imageio-client
python3-ovirt-engine-sdk4
"

cdi_importer_extra_aarch64="
ovirt-imageio-client
python3-ovirt-engine-sdk4
"

cdi_uploadserver="
libnbd
qemu-img
"

testimage="
crypto-policies-scripts
qemu-img
nginx
python3-systemd
systemd-libs
openssl
buildah
"

# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_x86_64 \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_x86_64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_x86_64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer \
    $cdi_importer_extra_x86_64

bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_x86_64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=${ARCHITECTURE} \
    //:bazeldnf -- prune

# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=aarch64 \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_aarch64 --arch aarch64 \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=aarch64 \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_aarch64 --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=aarch64 \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_aarch64 --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer \
    $cdi_importer_extra_aarch64

bazel run \
    --config=aarch64 \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_aarch64 --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=aarch64 \
    //:bazeldnf -- prune

# s390x #####
# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=s390x \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_s390x --arch s390x \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=s390x \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_s390x --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=s390x \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_s390x --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer

bazel run \
    --config=s390x \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_s390x --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=s390x \
    //:bazeldnf -- prune

