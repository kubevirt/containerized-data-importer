#!/bin/sh
set -e

export BUILD_ARCH=s390x,aarch64,x86_64

make bazel-push-images
