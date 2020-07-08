#!/bin/sh
CONFIG_FILE=${1:-/etc/docker/registry/registry-config.yml}
registry serve ${CONFIG_FILE}
