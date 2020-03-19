#!/bin/bash

set -euo pipefail

cp ./create_lvm.sh /host/create_lvm.sh
chroot /host ./create_lvm.sh
rm /host/create_lvm.sh

# let the monitoring script know we're done
echo "done" >/ready
echo "ready"
while true; do
  sleep 60
done
