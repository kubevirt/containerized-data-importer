#!/bin/sh
set -e
build_date="$(date +%Y%m%d)"
cat "$QUAY_PASSWORD" | docker login --username $(cat "$QUAY_USER") --password-stdin=true quay.io

export DOCKER_TAG="${build_date}_$(git show -s --format=%h)"
export BUILD_ARCH=s390x,aarch64,x86_64

make manifests
make bazel-push-images

base_url="kubevirt-prow/devel/nightly/release/kubevirt/containerized-data-importer"
bucket_dir="${base_url}/${build_date}"
gsutil cp ./_out/manifests/release/cdi-operator.yaml gs://$bucket_dir/cdi-operator.yaml
gsutil cp ./_out/manifests/release/cdi-cr.yaml gs://$bucket_dir/cdi-cr.yaml

echo ${build_date} > ./_out/build_date
gsutil cp ./_out/build_date gs://${base_url}/latest

git show -s --format=%H > ./_out/commit
gsutil cp ./_out/commit gs://${bucket_dir}/commit
