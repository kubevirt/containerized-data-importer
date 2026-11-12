#!/usr/bin/env bash

set -exuo pipefail

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source hack/build/common.sh

PLATFORM=$(uname -m)
case ${PLATFORM} in
x86_64* | i?86_64* | amd64*)
  ARCH="amd64"
  ;;
aarch64* | arm64*)
  ARCH="arm64"
  ;;
s390x)
  ARCH="s390x"
  ;;
ppc64le)
  ARCH="ppc64le"
  ;;
*)
  echo "invalid Arch, only support x86_64, aarch64 and s390x"
  exit 1
  ;;
esac

rm -rf "${TESTS_OUT_DIR}/ginkgo"
mkdir -p "${TESTS_OUT_DIR}"

GOOS=linux GOPROXY=${GOPROXY:-off} GOARCH=${ARCH} go_build -o "${TESTS_OUT_DIR}/ginkgo" "${CDI_DIR}/vendor/github.com/onsi/ginkgo/v2/ginkgo/main.go"
