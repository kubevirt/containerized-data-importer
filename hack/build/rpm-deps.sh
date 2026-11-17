#!/usr/bin/env bash

set -ex

source hack/build/common.sh
source hack/build/config.sh

# Freezing nginx to avoid segfaults when pulling images with tls
# May get fixed with 1.25
NGINX_VERSION="1:1.22.1-4.module_el9+666+132dc76f"

# CS10 uses its own repo file and *_cs10 tree names.
repo_file="repo.yaml"
name_suffix=""
if [[ "${CDI_CENTOS_STREAM_VERSION}" = "10" ]]; then
    repo_file="repo-cs10.yaml"
    name_suffix="_cs10"
    # nginx is not modular in el10
    NGINX_VERSION="2:1.26.3-16.el10"
fi

bazeldnf_repos="--repofile ${repo_file}"
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
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
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
ovirt-imageio-daemon
python3-ovirt-engine-sdk4
"

cdi_importer_extra_aarch64="
ovirt-imageio-client
ovirt-imageio-daemon
python3-ovirt-engine-sdk4
"

cdi_uploadserver="
libnbd
qemu-img
"

testimage="
crypto-policies-scripts
qemu-img
nginx-${NGINX_VERSION}
python3-systemd
systemd-libs
openssl
buildah
"

if [[ "${CDI_CENTOS_STREAM_VERSION}" = "10" ]]; then
    # openssl-libs requires fips-provider-so; pick openssl-fips-provider
    # explicitly instead of letting the solver choose fips-provider-next.
    centos_extra="${centos_extra} openssl-fips-provider"

    # containers-common-extra requires oci-runtime; select crun for test images.
    testimage="${testimage} crun"

    # EL10 ships the gzip filter in nbdkit-basic-filters.
    cdi_importer="${cdi_importer//nbdkit-gzip-filter/}"
    # python3-pycurl is only in EPEL 10. It is required by python3-ovirt-engine-sdk4,
    # so it goes with oVirt (x86_64 and aarch64), not with the common list or s390x.
    cdi_importer="${cdi_importer//python3-pycurl/}"
    cdi_importer_extra_x86_64="${cdi_importer_extra_x86_64} python3-pycurl"
    cdi_importer_extra_aarch64="${cdi_importer_extra_aarch64} python3-pycurl"
    # CS10 RPM pins (name-epoch:version-release).
    cdi_importer="${cdi_importer//qemu-img/qemu-img-18:10.1.0-28.el10}"
    cdi_importer="${cdi_importer//libnbd/libnbd-0:1.25.8-1.el10}"
    cdi_uploadserver="${cdi_uploadserver//qemu-img/qemu-img-18:10.1.0-28.el10}"
    cdi_uploadserver="${cdi_uploadserver//libnbd/libnbd-0:1.25.8-1.el10}"
    testimage="${testimage//qemu-img/qemu-img-18:10.1.0-28.el10}"
fi

# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_x86_64${name_suffix} \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_x86_64${name_suffix} \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_x86_64${name_suffix} \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer \
    $cdi_importer_extra_x86_64

bazel run \
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_x86_64${name_suffix} \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=${ARCHITECTURE} ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- prune

# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=aarch64 ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_aarch64${name_suffix} --arch aarch64 \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=aarch64 ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_aarch64${name_suffix} --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=aarch64 ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_aarch64${name_suffix} --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer \
    $cdi_importer_extra_aarch64

bazel run \
    --config=aarch64 ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_aarch64${name_suffix} --arch aarch64 \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=aarch64 ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- prune

# s390x #####
# XXX: passing --nobest otherwise we fail to solve the dependencies
bazel run \
    --config=s390x ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public \
    --name testimage_s390x${name_suffix} --arch s390x \
    --nobest \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $testimage

bazel run \
    --config=s390x ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name centos_base_s390x${name_suffix} --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra

bazel run \
    --config=s390x ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_importer_base_s390x${name_suffix} --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_importer

bazel run \
    --config=s390x ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- rpmtree \
    --public --nobest \
    --name cdi_uploadserver_base_s390x${name_suffix} --arch s390x \
    --basesystem centos-stream-release \
    ${bazeldnf_repos} \
    $centos_base \
    $centos_extra \
    $cdi_uploadserver

# remove all RPMs which are no longer referenced by a rpmtree
bazel run \
    --config=s390x ${BAZEL_CS_CONFIG} \
    //:bazeldnf -- prune
