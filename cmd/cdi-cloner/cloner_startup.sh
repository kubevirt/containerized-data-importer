#!/bin/sh

#Copyright 2018 The CDI Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

set -euo pipefail

if [[ -z "$VOLUME_MODE" ]]; then
    echo "VOLUME_MODE missing" 1>&2
    exit 1
fi

if [[ -z "$MOUNT_POINT" ]]; then
    echo "MOUNT_POINT missing" 1>&2
    exit 1
fi

echo "VOLUME_MODE=$VOLUME_MODE"
echo "MOUNT_POINT=$MOUNT_POINT"

if [ "$VOLUME_MODE" == "block" ]; then
    UPLOAD_BYTES=$(blockdev --getsize64 $MOUNT_POINT)
    echo "UPLOAD_BYTES=$UPLOAD_BYTES"

    /usr/bin/cdi-cloner -v=3 -alsologtostderr -content-type blockdevice-clone -upload-bytes $UPLOAD_BYTES -mount $MOUNT_POINT
else
    pushd $MOUNT_POINT
    UPLOAD_BYTES=$(du -sb . | cut -f1)
    echo "UPLOAD_BYTES=$UPLOAD_BYTES"

    /usr/bin/cdi-cloner -v=3 -alsologtostderr -content-type filesystem-clone -upload-bytes $UPLOAD_BYTES -mount $MOUNT_POINT

    popd
fi
