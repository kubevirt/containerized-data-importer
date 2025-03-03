#!/bin/sh
set -e
cat "$QUAY_PASSWORD" | docker login --username $(cat "$QUAY_USER") --password-stdin=true quay.io

export BUILD_ARCH=s390x,aarch64,x86_64

make bazel-push-images
