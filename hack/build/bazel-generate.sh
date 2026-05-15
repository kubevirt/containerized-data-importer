#!/usr/bin/env bash
set -e

source hack/build/common.sh
source hack/build/config.sh

# generate BUILD files
if [[ "$@" =~ "vendor" ]]; then
    rm -f vendor/github.com/grpc-ecosystem/grpc-gateway/v2/runtime/BUILD.bazel
    rm -f vendor/github.com/grpc-ecosystem/grpc-gateway/v2/utilities/BUILD.bazel
    rm -f vendor/github.com/grpc-ecosystem/grpc-gateway/v2/internal/httprule/BUILD.bazel
    rm -f vendor/github.com/google/cel-go/parser/gen/BUILD.bazel
fi

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
