#!/bin/bash

PODS_TIMEOUT=1800

sync_log=$(date +"sync%y%m%d%H%M.log")
k=cluster-up/kubectl.sh

function wait_pods_ready {
  echo_log "Waiting $PODS_TIMEOUT seconds for pods to be ready"
  wait_time=0
  not_ready_pods="something"
  prev_pods=""
  while [ -n "$not_ready_pods" ] && [ $wait_time -lt ${PODS_TIMEOUT} ]; do
    not_ready_pods=$($k get pods -A -o'custom-columns=metadata:metadata.name,status:status.containerStatuses[*].ready' --no-headers | grep false)
    if [ -n "$not_ready_pods" ] ; then
      if [ "$not_ready_pods" != "$prev_pods" ] ; then
        echo_log "Not ready pods: $not_ready_pods"
        prev_pods=$not_ready_pods
      fi
      wait_time=$((wait_time + 5))
      sleep 5
    fi
  done
  echo_log "Waited $wait_time seconds"
  if [ -z "$not_ready_pods" ] ; then
    echo_log "All pods are ready"
  else
    echo_log "Not all pods are ready"
  fi
}

function echo_log {
  s=$(date +"%H:%M:%S $1")
  echo $s
  echo $s >> $sync_log
}

function run_time_log {
  echo_log "$1"
  SECONDS=0
  $1
  echo_log "$SECONDS sec"
}

echo_log "========================================================================================="
run_time_log "make cluster-down"
run_time_log "make cluster-up"
wait_pods_ready
run_time_log "make cluster-sync-cdi"
wait_pods_ready
run_time_log "make cluster-sync-cdi"
wait_pods_ready
run_time_log "make cluster-sync-cdi"
wait_pods_ready
echo_log "========================================================================================="
run_time_log "make cluster-sync-test-infra"
wait_pods_ready
run_time_log "make cluster-sync-test-infra"
wait_pods_ready
run_time_log "make cluster-sync-test-infra"
wait_pods_ready
echo_log "========================================================================================="
run_time_log "make cluster-down"
run_time_log "make cluster-up"
wait_pods_ready
run_time_log "make cluster-sync"
wait_pods_ready
run_time_log "make cluster-sync"
wait_pods_ready
run_time_log "make cluster-sync"
wait_pods_ready
