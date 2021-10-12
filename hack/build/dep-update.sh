#!/usr/bin/env bash

set -e

export GO111MODULE=on

rm go.sum
go mod tidy
go mod vendor
