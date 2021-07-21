#!/usr/bin/env bash
set -e

source hack/build/common.sh
source hack/build/config.sh

# generate BUILD files
bazel run \
    --config=${ARCHITECTURE} \
    //:gazelle $@
