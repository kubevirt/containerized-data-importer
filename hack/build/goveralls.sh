#!/usr/bin/env bash
set -e

goveralls -service=github -coverprofile=.coverprofile
