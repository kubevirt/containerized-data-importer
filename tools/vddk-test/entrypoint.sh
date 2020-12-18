#!/usr/bin/env bash
set -m
set -x

mkfifo /tmp/vcfifo

/usr/bin/vcsim -l :8989 -E /tmp/vcfifo -dc 0 &
eval "$(cat /tmp/vcfifo)"

export GOVC_INSECURE=1
TESTSTORE=/tmp/teststore
mkdir -p $TESTSTORE

/usr/bin/govc datacenter.create testdc
/usr/bin/govc cluster.create testcluster
/usr/bin/govc cluster.add -hostname testhost -username user -password pass -noverify
/usr/bin/govc datastore.create -type local -name teststore -path $TESTSTORE testcluster/*

/usr/bin/govc vm.create testvm
while read line; do
    if [[ $line =~ (^[\s]*UUID:[\s]*)(.*)$ ]]; then
        uuid="${BASH_REMATCH[2]}"
        echo $uuid > /tmp/vmid
    fi
done < <(govc vm.info testvm)

/usr/bin/govc vm.disk.create -vm testvm -name testvm/testdisk.vmdk -size 2GB
while read line; do
    if [[ $line =~ (^[\s]*File:[\s]*)(.*)$ ]]; then
        file="${BASH_REMATCH[2]}"
        echo $file > /tmp/vmdisk
    fi
done < <(govc device.info -vm testvm)

/usr/bin/govc snapshot.create -vm testvm Snapshot-1
/usr/bin/govc snapshot.create -vm testvm Snapshot-2
while read line; do
    if [[ $line =~ \[(.*?)\].*Snapshot-1 ]]; then
        snapshot1="${BASH_REMATCH[1]}"
        echo $snapshot1 > /tmp/vmsnapshot1
    fi
    if [[ $line =~ \[(.*?)\].*Snapshot-2 ]]; then
        snapshot2="${BASH_REMATCH[1]}"
        echo $snapshot2 > /tmp/vmsnapshot2
    fi
done < <(govc snapshot.tree -vm testvm -i)

fg
