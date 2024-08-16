#!/usr/bin/env bash

set -ex

source hack/build/common.sh
source hack/build/config.sh

rm -rf "${TESTS_OUT_DIR}/ginkgo"

bazel build \
    --verbose_failures \
    --config=${ARCHITECTURE} \
    //vendor/github.com/onsi/ginkgo/v2/ginkgo:ginkgo

bazel run \
    --verbose_failures \
    --config=${ARCHITECTURE} \
    :build-ginkgo -- ${TESTS_OUT_DIR}/ginkgo
