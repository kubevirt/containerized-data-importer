#!/usr/bin/env bash

set -e

export GO111MODULE=on

(
    cd staging/src/kubevirt.io/containerized-data-importer-api
    rm -f go.sum
    go mod tidy
)

rm -f go.sum
go mod tidy
go mod vendor

(
    cd vendor/kubevirt.io
    rm -rf containerized-data-importer-api
    ln -s ../../staging/src/kubevirt.io/containerized-data-importer-api containerized-data-importer-api
)
