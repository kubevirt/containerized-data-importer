#!/usr/bin/env bash

# generate BUILD files
bazel run \
    --platforms=@io_bazel_rules_go//go/toolchain:linux_amd64_cgo \
    //:gazelle $@
