#!/bin/sh
set -e
build_date="$(date +%Y%m%d)"
cat "$QUAY_PASSWORD" | docker login --username $(cat "$QUAY_USER") --password-stdin=true quay.io

case $BUILD_ARCH in
  crossbuild-s390x|s390x)
    ARCH_SUFFIX="-s390x"
    ;;
  crossbuild-aarch64|aarch64)
    ARCH_SUFFIX="-arm64"
    ;;
  *)
    echo "No BUILD_ARCH or ${BUILD_ARCH}, assuming amd64"
    ;;
esac

export DOCKER_TAG="${build_date}_$(git show -s --format=%h)${ARCH_SUFFIX}"

make manifests
make bazel-push-images

base_url="kubevirt-prow/devel/nightly/release/kubevirt/containerized-data-importer"
bucket_dir="${base_url}/${build_date}"
gsutil cp ./_out/manifests/release/cdi-operator.yaml gs://$bucket_dir/cdi-operator${ARCH_SUFFIX}.yaml
gsutil cp ./_out/manifests/release/cdi-cr.yaml gs://$bucket_dir/cdi-cr${ARCH_SUFFIX}.yaml

echo ${build_date} > ./_out/build_date
gsutil cp ./_out/build_date gs://${base_url}/latest${ARCH_SUFFIX}

git show -s --format=%H > ./_out/commit
gsutil cp ./_out/commit gs://${base_url}/commit${ARCH_SUFFIX}
