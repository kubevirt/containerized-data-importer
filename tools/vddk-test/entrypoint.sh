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

fg
