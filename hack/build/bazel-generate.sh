#!/usr/bin/env bash
set -e

source hack/build/common.sh
source hack/build/config.sh

# generate BUILD files
bazel run \
    --config=${ARCHITECTURE} \
    //:gazelle $@

if [[ "$@" =~ "vendor" ]]; then
  bazel run \
      --config=${ARCHITECTURE} \
      -- @com_github_bazelbuild_buildtools//buildozer 'add clinkopts -lnbd' //$HOME/go/src/kubevirt.io/containerized-data-importer/vendor/libguestfs.org/libnbd/:go_default_library
 
  bazel run \
      --config=${ARCHITECTURE} \
      -- @com_github_bazelbuild_buildtools//buildozer 'add copts -D_GNU_SOURCE=1' //$HOME/go/src/kubevirt.io/containerized-data-importer/vendor/libguestfs.org/libnbd/:go_default_library
fi
