#!/usr/bin/env bash

set -euo pipefail

# bazel will fail if either HOME or USER are not set
HOME=$(pwd)
export HOME
USER='kubeadmin'
export USER

source cluster-sync/install.sh
