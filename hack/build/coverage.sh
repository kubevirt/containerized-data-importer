#!/usr/bin/env bash

set -eu

COVER_DIR="$(mktemp -d)"
go tool cover -o "${COVER_DIR}/index.html" -html ".coverprofile"

PORT="8000"
python3 -m http.server ${PORT} --directory ${COVER_DIR}
