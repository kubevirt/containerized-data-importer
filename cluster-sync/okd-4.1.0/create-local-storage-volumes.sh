#!/usr/bin/env bash
if [ ! -e /mnt/local-storage/local/disk1 ]; then
  # Create local-volume directories
  for i in {1..10}
  do
    sudo mkdir -p /var/local/kubevirt-storage/local-volume/disk${i}
    sudo mkdir -p /mnt/local-storage/local-sc/disk${i}
    sudo mount --bind /var/local/kubevirt-storage/local-volume/disk${i} /mnt/local-storage/local-sc/disk${i}
  done
  sudo chmod -R 777 /var/local/kubevirt-storage/local-volume
  # Setup selinux permissions to local volume directories.
  sudo chcon -R unconfined_u:object_r:svirt_sandbox_file_t:s0 /mnt/local-storage/
fi