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

DOCKER=1
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
	${DO} "./hack/build/build-go.sh clean; rm -rf bin/* _out/* manifests/generated/* .coverprofile release-announcement"

generate:
	${DO} "./hack/update-codegen.sh"; make bazel-generate

generate-verify:
	${DO} "./hack/verify-codegen.sh"

gomod-update:
	SYNC_VENDOR=true ${DO_BAZ} "./hack/build/dep-update.sh"

deps-update: gomod-update bazel-generate

apidocs:
	${DO} "./hack/update-codegen.sh && ./hack/gen-swagger-doc/gen-swagger-docs.sh v1alpha1 html"

build-functest:
	${DO} ./hack/build/build-functest.sh

# WHAT must match go tool style package paths for test targets (e.g. ./path/to/my/package/...)
test: test-unit test-functional test-lint

test-unit: WHAT = ./pkg/... ./cmd/...
test-unit:
	${DO} "./hack/build/run-unit-tests.sh ${WHAT}"

test-functional:  WHAT = ./tests/...
test-functional: build-functest
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

# test-lint runs gofmt and golint tests against src files
test-lint:
	${DO} "./hack/build/run-lint-checks.sh"

docker-registry-cleanup: 
	./hack/build/cleanup_docker.sh 

publish: manifests push

vet:
	${DO} "./hack/build/build-go.sh vet ${WHAT}"

format:
	${DO} "./hack/build/format.sh"

manifests:
	${DO} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} NAMESPACE=${NAMESPACE} ./hack/build/build-manifests.sh"

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
	./cluster-sync/clean.sh

cluster-sync: cluster-clean
	./cluster-sync/sync.sh CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}

bazel-generate:
	SYNC_VENDOR=true ${DO_BAZ} "./hack/build/bazel-generate.sh -- pkg/ tools/ tests/ cmd/ vendor/"

bazel-cdi-generate:
	${DO_BAZ} "./hack/build/bazel-generate.sh -- pkg/ tools/ tests/ cmd/"

bazel-build:
	${DO_BAZ} "./hack/build/bazel-build.sh"

bazel-build-images:	bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-build-images.sh"

bazel-push-images: bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-push-images.sh"

push: bazel-push-images

builder-push:
	./hack/build/bazel-build-builder.sh

help:
	@echo "Usage: make [Targets ...]"
	@echo " all "
	@echo "  : cleans up previous build artifacts, compiles all CDI packages and builds containers"
	@echo " apidocs "
	@echo "  : generate client-go code (same as 'make generate') and swagger docs."
	@echo " build "
	@echo "  : compile all CDI binary artifacts and generate controller and operator manifests"
	@echo " build-functest-file-image-init "
	@echo "  : build the init container for the testing file server. (NOTE: the http and s3 components contain no CDI code, so do no require a build)"
	@echo " build-functest-image-http "
	@echo "  : build the http container for the testing file server"
	@echo " build-functest-registry-init "
	@echo "  : build the init container for the testing docker registry server"
	@echo " docker-functest-registry-populate "
	@echo "  : build the container that popuplates registry server with various container images"
	@echo " docker-functest-registry "
	@echo "  : build the container that hosts docker registry"
	@echo " clean "
	@echo "  : cleans up previous build artifacts"
	@echo " cluster-up "
	@echo "  : start a default Kubernetes or Open Shift cluster. set KUBEVIRT_PROVIDER environment variable to either 'k8s-1.16.2' or 'os-3.11.0' to select the type of cluster. set KUBEVIRT_NUM_NODES to something higher than 1 to have more than one node."
	@echo " cluster-down "
	@echo "  : stop the cluster, doing a make cluster-down && make cluster-up will basically restart the cluster into an empty fresh state."
	@echo " cluster-down-purge "
	@echo "  : cluster-down and cleanup all cached images from docker registry. Accepts [make variables](#make-variables) DOCKER_PREFIX. Removes all images of the specified repository. If not specified removes localhost repository of current cluster instance."
	@echo " cluster-sync "
	@echo "  : builds the controller/importer/cloner, and pushes it into a running cluster. The cluster must be up before running a cluster sync. Also generates a manifest and applies it to the running cluster after pushing the images to it."
	@echo " deps-update "
	@echo "  : runs 'go mod tidy' and 'go mod vendor'"
	@echo " docker "
	@echo "  : compile all binaries and build all containerized"
	@echo " format "
	@echo "  : execute 'shfmt', 'goimports', and 'go vet' on all CDI packages.  Writes back to the source files."
	@echo " generate "
	@echo "  : generate client-go deepcopy functions, clientset, listers and informers."
	@echo " generate-verify "
	@echo "  : generate client-go deepcopy functions, clientset, listers and informers and validate codegen."
	@echo " goveralls "
	@echo "  : run code coverage tracking system."
	@echo " manifests "
	@echo "  : generate a cdi-controller and operator manifests in '_out/manifests/'.  Accepts [make variables]\(#make-variables\) DOCKER_TAG, DOCKER_PREFIX, VERBOSITY, PULL_POLICY, CSV_VERSION, QUAY_REPOSITORY, QUAY_NAMESPACE"
	@echo " push "
	@echo "  : compiles, builds, and pushes to the repo passed in 'DOCKER_PREFIX=<my repo>'"
	@echo " release-description "
	@echo "  : generate a release announcement detailing changes between 2 commits (typically tags).  Expects 'RELREF' and 'PREREF' to be set"
	@echo " test "
	@echo "  : execute all tests (_NOTE:_ 'WHAT' is expected to match the go cli pattern for paths e.g. './pkg/...'.  This differs slightly from rest of the 'make' targets)"
	@echo " vet	"
	@echo "  : lint all CDI packages"

.DEFAULT_GOAL := help
