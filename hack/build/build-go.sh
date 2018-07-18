#!/usr/bin/env bash

set -e

source hack/build/common.sh
source hack/build/config.sh

mkdir -p ${BIN_DIR}
mkdir -p ${CMD_OUT_DIR}

if [ -z "$1" ]; then
	go_opt="build"
else
	go_opt=$1
	shift
fi

targets="$@"

if [ "${go_opt}" == "test" ]; then
	if [ -z "${targets}" ]; then
        targets="${CDI_PKGS}"
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
		BIN_NAME=$(basename $tgt)
		if [[ "${BIN_NAME}" == "${CLONER}" ]]; then
		    continue
		fi
		rm -f ${CMD_OUT_DIR}/${BIN_NAME}/${BIN_NAME}
		rm -f ${BIN_DIR}/${BIN_NAME}
		(
			cd $tgt

            # Only build executables for linux amd64
			GOOS=linux GOARCH=amd64 go build -o ${CMD_OUT_DIR}/${BIN_NAME}/${BIN_NAME} -ldflags '-extldflags "static"'

			ln -sf ${BIN_NAME} ${CDI_DIR}/bin/${BIN_NAME}
		)
	done
elif [ "$go_opt" == "vet" ]; then
    if [ -z "${targets}" ]; then
        # Do not vet vendor or pkg/client (contains known error)
        # To be fixed by https://github.com/kubernetes/kubernetes/pull/60584
        targets=$(sed "s,kubevirt.io/containerized-data-importer,${CDI_DIR},g" <(go list ./... | grep -v -E "vendor|pkg/client" | sort -u ))
    fi
    for tgt in ${targets}; do
        (
            cd "${tgt}"
            go vet -v ./...
        )
    done
else # Pass go commands directly on to packages expect vendor
    if [ -z ${targets} ]; then
        targets="$(go list ./... | grep -v "vendor" | sort -u)" # pkg/client is generated code, ignore it
    fi
    for tgt in ${targets}; do
        (
            cd $tgt
            go ${go_opt} ./...
        )
    done
fi
