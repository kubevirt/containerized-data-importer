#!/usr/bin/env bash

set -exuo pipefail

GIT_ASKPASS="$(pwd)/automation/git-askpass.sh"
[ -f "$GIT_ASKPASS" ] || exit 1
export GIT_ASKPASS

export DOCKER_TAG=""

make apidocs
make manifests
make build-functests

bash hack/gen-swagger-doc/deploy.sh
bash hack/publish-staging.sh
