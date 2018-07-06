#!/usr/bin/env bash

source hack/build/common.sh

dirs="${BIN_DIR} ${CMD_OUT_DIR}"

for dir in ${dirs};do
    (
        cd $dir
        rm -rf cdi-*
    )
done
