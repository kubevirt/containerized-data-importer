#!/bin/bash

set -euo pipefail

DEVICE_SIZE=${DEVICE_SIZE:-30G}
DATA_DIR=${DATA_DIR:-"/data"}

echo $DEVICE_SIZE
echo $DATA_DIR

if [ -d $DATA_DIR ]; then
  echo "Creating loop back device"
  if [ -e "/dev/loop0" ]; then
    losetup -d /dev/loop0
  fi
  rm -rf /dev/loop0
  mknod -m 0660 /dev/loop0 b 7 0
  truncate -s $DEVICE_SIZE $DATA_DIR/ember-volumes
  loop_device=$(losetup --show -f $DATA_DIR/ember-volumes)
  echo "Loop device: $loop_device"
  pvcreate $loop_device -vv
  vgcreate ember-volumes $loop_device -vv
  vgscan
else
  echo "Data directory not found, exiting"
  exit 1
fi
