#!/usr/bin/env bash

source ./common.sh

shfmt -i 4 -w "${CDI_DIR}/hack manifests/cloner/" #TODO new path
goimports -w -local kubevirt.io ${CDI_DIR}/cmd/ ${CDI_DIR}/pkg/ ${CDI_DIR}/test/
( cd ${CDI_DIR} && go vet ./cmd/... ./pkg/... ./test/... )
