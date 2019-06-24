#!/bin/bash
set -eu

ARG1=${1-}
if [ "$ARG1" = "minikube" ]; then
    TEST_NAMESPACE="marketplace"
else
    TEST_NAMESPACE="openshift-marketplace"
fi

# Run the tests through the operator-sdk
echo "Running operator-sdk test"
operator-sdk test local ./test/e2e/ --no-setup --go-test-flags -v --namespace $TEST_NAMESPACE
