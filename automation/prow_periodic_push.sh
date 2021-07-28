#!/bin/sh
set -e
build_date="$(date +%Y%m%d)"
cat "$QUAY_PASSWORD" | docker login --username $(cat "$QUAY_USER") --password-stdin=true quay.io
export DOCKER_TAG="${build_date}_$(git show -s --format=%h)-arm64"
make manifests
make bazel-push-images
bucket_dir="kubevirt-prow/devel/nightly/release/kubevirt/containerized-data-importer/${build_date}"
gsutil cp ./_out/manifests/release/cdi-operator.yaml gs://$bucket_dir/cdi-operator-arm64.yaml
gsutil cp ./_out/manifests/release/cdi-cr.yaml gs://$bucket_dir/cdi-cr-arm64.yaml
