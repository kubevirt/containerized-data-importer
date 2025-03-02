#!/bin/sh -ex

GOLANGCI_VERSION="${GOLANGCI_VERSION:-v1.60.3}"
MONITORING_LINTER_VERSION="${MONITORING_LINTER_VERSION:-e2be790}"

go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}"
go install "github.com/kubevirt/monitoring/monitoringlinter/cmd/monitoringlinter@${MONITORING_LINTER_VERSION}"

golangci-lint  run ./... "$@"
monitoringlinter ./...
