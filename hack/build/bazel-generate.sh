#!/usr/bin/env bash
set -e

source hack/build/common.sh
source hack/build/config.sh

# generate BUILD files
bazel run \
    --config=${HOST_ARCHITECTURE} \
    //:gazelle $@

if [[ "$@" =~ "vendor" ]]; then
    bazel run \
        --config=${HOST_ARCHITECTURE} \
        -- :buildozer 'add clinkopts -lnbd' //vendor/libguestfs.org/libnbd/:go_default_library

    bazel run \
        --config=${HOST_ARCHITECTURE} \
        -- :buildozer 'add copts -D_GNU_SOURCE=1' //vendor/libguestfs.org/libnbd/:go_default_library
fi
