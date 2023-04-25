#!/bin/sh -ex

GOLANGCI_VERSION="${GOLANGCI_VERSION:-v1.52.2}"

go install "github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_VERSION}"

golangci-lint run --timeout=8m ./...
