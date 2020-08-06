#!/bin/bash
set -e

source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/ephemeral-provider-common.sh

LOCAL="localhost"
REPO=${DOCKER_PREFIX:-$LOCAL}

function conditionLog() {
    err=$1
    errmsg=$2
    msg=$3
    if [ $err -ne 0 ]; then
        echo $errmsg
    else
        echo $msg
    fi
}

function usage() {
    echo "USAGE: cleanup_docker.sh [DOCKER_PREFIX=<repo to purge>]"
}

function setRepo() {
    if [ "$REPO" = $LOCAL ] && [ "$DOCKER_PREFIX" = "" ]; then
        registry_port=$(_port registry)
        if [ -n "$registry_port" ] && [ "$registry_port" -eq "$registry_port" ] 2>/dev/null; then
            REPO=$LOCAL":"$registry_port
        else
            echo "Error on retrieving registry port on localhost. The cluster is probably down."
            usage
            exit 0
        fi
    fi
}

function dockerCleanup() {
    images=$(docker image ls | grep $REPO | awk '{print $3}')
    names=$(docker image ls | grep $REPO | awk '{print $1}')

    if [ "$images" == "" ]; then
        echo "No matching images for repo "$REPO
        exit 0
    fi

    count=0
    arr=($names)
    for image in $images; do
        docker rmi -f $image >/dev/null 2>&1
        conditionLog $? "Failed to remove "${arr[$count]} ${arr[$count]}
        count=$count+1
    done
}

echo "Setting repo"
setRepo
echo "Cleaning up"
dockerCleanup
