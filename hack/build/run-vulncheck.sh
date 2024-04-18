#!/bin/sh -eu

GOVULNCHECK_VERSION="${GOVULNCHECK_VERSION:-latest}"

go install "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}"
govulncheck -version ./...
