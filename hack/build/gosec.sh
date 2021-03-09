#!/bin/bash
set -e

source hack/build/common.sh
source hack/build/config.sh

export GOSECOUT=${OUT_DIR}/gosec

mkdir -p $GOSECOUT

cd $CDI_DIR/pkg
echo "Running gosec.."
gosec -sort -no-fail -quiet -out=${GOSECOUT}/junit-gosec.xml -fmt=junit-xml ./...