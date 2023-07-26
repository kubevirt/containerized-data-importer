#!/usr/bin/env bash
set -xeuo pipefail

export DOCKER_TAG="latest"
export KUBEVIRT_PROVIDER=external

echo "calling cluster-up to prepare config and check whether cluster is reachable"
bash -x ./cluster-up/up.sh

echo "deploying"
bash -x ./hack/cluster-deploy.sh

echo "testing"
mkdir -p "$ARTIFACT_DIR"
FUNC_TEST_ARGS='--ginkgo.timeout=24h --ginkgo.no-color --ginkgo.focus=\[crit:high\] --ginkgo.junit-report='"$ARTIFACT_DIR"'/junit.functest.xml' \
    bash -x ./hack/functests.sh
