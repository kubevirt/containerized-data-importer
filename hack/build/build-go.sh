#!/usr/bin/env bash

set -e

source hack/build/common.sh
source hack/build/config.sh

if [ -z "$1" ]; then
	go_opt="build"
else
	go_opt=$1
	shift
fi

targets="$@"

if [ "${go_opt}" == "test" ]; then
	if [ -z "${targets}" ]; then
        targets="${TESTS}"
	fi
	for tgt in ${targets}; do
			(
				cd $tgt
				go test -v ./...
			)
	done
elif [ "${go_opt}" == "build" ]; then
    if [ -z "${targets}" ]; then
        targets="${BINARIES}"
    fi
	for tgt in ${targets}; do
		eval "$(go env)"
		BIN_NAME=$(basename $tgt)
		rm -f ${CMD_OUT_DIR}/${BIN_NAME}/${BIN_NAME}
		rm -f ${BIN_DIR}/${BIN_NAME}
		(
			cd $tgt

			go vet ./...

            # Only build executables for linux amd64
			GOOS=linux GOARCH=amd64 go build -o ${CMD_OUT_DIR}/${BIN_NAME}/${BIN_NAME} -ldflags '-extldflags "static"'

			ln -sf ${BIN_NAME} ${CDI_DIR}/bin/${BIN_NAME}
		)
	done
else # Pass go commands directly on to packages
    if [ -z ${targets} ]; then
        targets="$(go list ./... | grep -v 'pkg/client')" # pkg/client is generated code, ignore it
    fi
    for tgt in ${targets}; do
        go "${go_opt}" ./...
    done
fi
