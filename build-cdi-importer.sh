#!/bin/bash

cid=`docker ps |grep entrypoint-bazel |awk '{print $1}'`
docker exec -it $cid bash -c "cd /root/go/src/kubevirt.io/containerized-data-importer/cmd/cdi-importer/ && source /etc/profile && go build ./"
cp /var/lib/docker/volumes/kubevirt-cdi-volume/_data/go/src/kubevirt.io/containerized-data-importer/cmd/cdi-importer/cdi-importer ./
