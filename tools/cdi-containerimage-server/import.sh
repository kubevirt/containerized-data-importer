#!/usr/bin/env bash

set -e

port=${1:-"8100"}
wait_file=${2:-"/shared/ready"}
done_file=${3:-"/shared/done"}
image_name=${4:-"disk.img"}
out_file=${5:-"/data/disk.img"}

while ! [ -f ${wait_file} ]; do
    echo "Waiting for cdi-containerimage-server"
    sleep 2;
done

export IMPORTER_ENDPOINT="http://localhost:${port}/${image_name}"
export IMPORTER_SOURCE="http"

/usr/bin/cdi-importer

echo cdi-importer exited with $?
touch "$done_file"
