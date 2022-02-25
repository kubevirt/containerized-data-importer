#!/bin/sh
#
# Check for duplicate SHA256 files in WORKSPACE.
#

CDI_DIR="$(cd $(dirname $0)/../../ && pwd -P)"

DUPLICATES=$(grep sha256 "${CDI_DIR}/WORKSPACE" | sort | uniq -d)
if [ ! -z "${DUPLICATES}" ]; then
    echo "Found duplicate SHA256 lines in WORKSPACE."
    echo "${DUPLICATES}"
    echo "Please make sure you are fetching a different file for other architectures!"
    exit 1
fi

