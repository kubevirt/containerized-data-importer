#!/bin/bash

set -euo pipefail

DEVICE_SIZE=${DEVICE_SIZE:-30G}
DATA_DIR=${DATA_DIR:-"/data"}

#microdnf install compat-readline5 targetcli

if [ -d $DATA_DIR ]; then
  if [ -e "/dev/loop0" ]; then
    losetup -d /dev/loop0
  fi
  rm -rf /dev/loop0
  mknod -m 0660 /dev/loop0 b 7 0
  truncate -s $DEVICE_SIZE $DATA_DIR/ember-volumes
  loop_device=$(losetup --show -f $DATA_DIR/ember-volumes)
  pvcreate $loop_device
  vgcreate ember-volumes $loop_device
  vgscan
else
  echo "Data directory not found, exiting"
  exit 1
fi

# let the monitoring script know we're done
echo "done" >/ready
echo "ready"
while true; do
  sleep 60
done
