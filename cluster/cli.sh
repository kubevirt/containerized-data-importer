#!/bin/bash -e

source ./cluster/gocli.sh

if [[ -t 1 ]]; then
    $gocli_interactive "$@"
else
    $gocli "$@"
fi

