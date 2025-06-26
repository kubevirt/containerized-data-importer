#!/usr/bin/env bash

set -e

script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/build/common.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh

cri_bin=$1
if [ -z $cri_bin ]; then
    cri_bin=$CDI_CRI
fi

if [ -n "${EXTRA_IMAGES}" ]; then
    registry_port=$(_port registry)
    for img in ${EXTRA_IMAGES}; do
        base_image_path="${img#*/}"
        local_img_url="localhost:${registry_port}/${base_image_path}"
        $cri_bin pull $img
        $cri_bin tag $img $local_img_url
        $cri_bin push $local_img_url
    done
fi
