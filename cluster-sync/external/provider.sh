#!/usr/bin/env bash

source cluster-sync/install.sh

function _kubectl(){
  kubectl "$@"
}

function seed_images(){
  echo "seed_images is a noop for external provider"
}

function verify() {
  echo "Verify not needed for external provider"
}


function up() {
  echo "using external provider"
}

function configure_local_storage() {
  echo "Local storage not needed for external provider..."
}


