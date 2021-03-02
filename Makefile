#Copyright 2018 The CDI Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.

.PHONY: manifests \
		cluster-up cluster-down cluster-sync \
		test test-functional test-unit test-lint \
		publish \
		vet \
		format \
		goveralls \
		release-description \
		bazel-generate bazel-build bazel-build-images bazel-push-images

DOCKER?=1
ifeq (${DOCKER}, 1)
	# use entrypoint.sh (default) as your entrypoint into the container
	DO=./hack/build/in-docker.sh
	# use entrypoint-bazel.sh as your entrypoint into the container.
	DO_BAZ=./hack/build/bazel-docker.sh
else
	DO=eval
	DO_BAZ=eval
endif

all: manifests bazel-build-images

clean:
	${DO_BAZ} "./hack/build/build-go.sh clean; rm -rf bin/* _out/* manifests/generated/* .coverprofile release-announcement"

update-codegen:
	${DO_BAZ} "./hack/update-codegen.sh"

generate: update-codegen bazel-generate

generate-verify:
	${DO_BAZ} "./hack/verify-codegen.sh"

gomod-update:
	${DO_BAZ} "./hack/build/dep-update.sh"

deps-update: gomod-update bazel-generate

apidocs:
	${DO_BAZ} "./hack/update-codegen.sh && ./hack/gen-swagger-doc/gen-swagger-docs.sh v1beta1 html"

build-functest:
	${DO_BAZ} ./hack/build/build-functest.sh

# WHAT must match go tool style package paths for test targets (e.g. ./path/to/my/package/...)
test: test-unit test-functional test-lint

test-unit: WHAT = ./pkg/... ./cmd/...
test-unit:
	${DO_BAZ} "./hack/build/run-unit-tests.sh ${WHAT}"

test-functional:  WHAT = ./tests/...
test-functional: build-functest
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

# test-lint runs gofmt and golint tests against src files
test-lint:
	${DO_BAZ} "./hack/build/run-lint-checks.sh"

docker-registry-cleanup:
	./hack/build/cleanup_docker.sh

publish: manifests push

vet:
	${DO_BAZ} "./hack/build/build-go.sh vet ${WHAT}"

format:
	${DO_BAZ} "./hack/build/format.sh"

manifests:
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} CR_NAME=${CR_NAME} NAMESPACE=${NAMESPACE} ./hack/build/build-manifests.sh"

goveralls: test-unit
	${DO} "TRAVIS_JOB_ID=${TRAVIS_JOB_ID} TRAVIS_PULL_REQUEST=${TRAVIS_PULL_REQUEST} TRAVIS_BRANCH=${TRAVIS_BRANCH} ./hack/build/goveralls.sh"

release-description:
	./hack/build/release-description.sh ${RELREF} ${PREREF}

cluster-up:
	./cluster-up/up.sh

cluster-down:
	./cluster-up/down.sh

cluster-down-purge: docker-registry-cleanup cluster-down

cluster-clean:
	CDI_CLEAN="all" ./cluster-sync/clean.sh

cluster-clean-cdi:
	./cluster-sync/clean.sh

cluster-clean-test-infra:
	CDI_CLEAN="test-infra" ./cluster-sync/clean.sh

cluster-sync-cdi: cluster-clean-cdi
	./cluster-sync/sync.sh CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}

cluster-sync-test-infra: cluster-clean-test-infra
	CDI_SYNC="test-infra" ./cluster-sync/sync.sh CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}

cluster-sync: cluster-sync-cdi cluster-sync-test-infra

bazel-generate:
	${DO_BAZ} "./hack/build/bazel-generate.sh -- pkg/ tools/ tests/ cmd/ vendor/"

bazel-cdi-generate:
	${DO_BAZ} "./hack/build/bazel-generate.sh -- pkg/ tools/ tests/ cmd/"

bazel-build:
	${DO_BAZ} "./hack/build/bazel-build.sh"

gosec:
	${DO_BAZ} "GOSEC=${GOSEC} ./hack/build/gosec.sh"

bazel-build-images:	bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-build-images.sh"

bazel-push-images: bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} DOCKER_CA_CERT_FILE=${DOCKER_CA_CERT_FILE} ./hack/build/bazel-push-images.sh"

push: bazel-push-images

builder-push:
	./hack/build/bazel-build-builder.sh

openshift-ci-image-push:
	./hack/build/osci-image-builder.sh

help:
	@echo "Usage: make [Targets ...]"
	@echo " all "
	@echo "  : cleans up previous build artifacts, compiles all CDI packages and builds containers"
	@echo " apidocs "
	@echo "  : generate client-go code (same as 'make generate') and swagger docs."
	@echo " build-functest "
	@echo "  : build the functional tests (content of tests/ subdirectory)."
	@echo " bazel-build "
	@echo "  : build all the Go binaries used."
	@echo " bazel-build-images "
	@echo "  : build all the container images used (for both CDI and functional tests)."
	@echo " bazel-generate "
	@echo "  : generate BUILD files for Bazel."
	@echo " bazel-push-images "
	@echo "  : push the built container images to the registry defined in DOCKER_PREFIX"
	@echo " builder-push "
	@echo "  : Build and push the builder container image, declared in docker/builder/Dockerfile."
	@echo " clean "
	@echo "  : cleans up previous build artifacts"
	@echo " cluster-up "
	@echo "  : start a default Kubernetes or Open Shift cluster. set KUBEVIRT_PROVIDER environment variable to either 'k8s-1.18' or 'os-3.11.0' to select the type of cluster. set KUBEVIRT_NUM_NODES to something higher than 1 to have more than one node."
	@echo " cluster-down "
	@echo "  : stop the cluster, doing a make cluster-down && make cluster-up will basically restart the cluster into an empty fresh state."
	@echo " cluster-down-purge "
	@echo "  : cluster-down and cleanup all cached images from docker registry. Accepts [make variables](#make-variables) DOCKER_PREFIX. Removes all images of the specified repository. If not specified removes localhost repository of current cluster instance."
	@echo " cluster-sync "
	@echo "  : builds the controller/importer/cloner, and pushes it into a running cluster. The cluster must be up before running a cluster sync. Also generates a manifest and applies it to the running cluster after pushing the images to it."
	@echo " deps-update "
	@echo "  : runs 'go mod tidy' and 'go mod vendor'"
	@echo " format "
	@echo "  : execute 'shfmt', 'goimports', and 'go vet' on all CDI packages.  Writes back to the source files."
	@echo " generate "
	@echo "  : generate client-go deepcopy functions, clientset, listers and informers."
	@echo " generate-verify "
	@echo "  : generate client-go deepcopy functions, clientset, listers and informers and validate codegen."
	@echo " gomod-update "
	@echo "  : Update vendored Go code in vendor/ subdirectory."
	@echo " goveralls "
	@echo "  : run code coverage tracking system."
	@echo " manifests "
	@echo "  : generate a cdi-controller and operator manifests in '_out/manifests/'.  Accepts [make variables]\(#make-variables\) DOCKER_TAG, DOCKER_PREFIX, VERBOSITY, PULL_POLICY, CSV_VERSION, QUAY_REPOSITORY, QUAY_NAMESPACE"
	@echo " openshift-ci-image-push "
	@echo "  : Build and push the OpenShift CI build+test container image, declared in hack/ci/Dockerfile.ci"
	@echo " push "
	@echo "  : compiles, builds, and pushes to the repo passed in 'DOCKER_PREFIX=<my repo>'"
	@echo " release-description "
	@echo "  : generate a release announcement detailing changes between 2 commits (typically tags).  Expects 'RELREF' and 'PREREF' to be set"
	@echo " test "
	@echo "  : execute all tests (_NOTE:_ 'WHAT' is expected to match the go cli pattern for paths e.g. './pkg/...'.  This differs slightly from rest of the 'make' targets)"
	@echo " test-unit "
	@echo "  : Run unit tests."
	@echo " test-lint "
	@echo "  : Run gofmt and golint against src files"
	@echo " test-functional "
	@echo "  : Run functional tests (in tests/ subdirectory)."
	@echo " vet	"
	@echo "  : lint all CDI packages"

.DEFAULT_GOAL := help
