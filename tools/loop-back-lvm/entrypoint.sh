#!/bin/bash

set -euo pipefail

cp ./create_lvm.sh /host/create_lvm.sh
chroot /host ./create_lvm.sh
rm /host/create_lvm.sh
