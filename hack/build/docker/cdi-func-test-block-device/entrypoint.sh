#!/bin/bash

set -euo pipefail

rm -f loop0
rm -f loop1

dd if=/dev/zero of=loop0 bs=50M count=10
dd if=/dev/zero of=loop1 bs=50M count=10

if [ -e "/dev/loop0" ]; then
  losetup -d /dev/loop0
fi
if [ -e "/dev/loop1" ]; then
  losetup -d /dev/loop1
fi

rm -rf /dev/loop0
rm -rf /dev/loop1

mknod -m 0660 /dev/loop0 b 7 0
mknod -m 0660 /dev/loop1 b 7 1

losetup /dev/loop0 loop0
losetup /dev/loop1 loop1

if [ -e "/local-storage/block-device/loop0" ]; then
  unlink /local-storage/block-device/loop0
fi

if [ -e "/local-storage/block-device1/loop1" ]; then
  unlink /local-storage/block-device1/loop1
fi

ln -s /dev/loop0 /local-storage/block-device
ln -s /dev/loop1 /local-storage/block-device1

# for some reason without sleep, container sometime fails to create the file
sleep 10

# let the monitoring script know we're done
echo "done" >/ready
while true; do

    sleep 60
done

