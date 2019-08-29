#!/usr/bin/env bash
set -e

goveralls -service=travis-ci -coverprofile=.coverprofile
