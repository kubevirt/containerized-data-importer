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

deps-update:
	${DO_BAZ} "./hack/build/dep-update.sh"; make bazel-generate

apidocs:
	${DO} "./hack/update-codegen.sh && ./hack/gen-swagger-doc/gen-swagger-docs.sh v1alpha1 html"

build-functest:
	${DO} ./hack/build/build-functest.sh

# WHAT must match go tool style package paths for test targets (e.g. ./path/to/my/package/...)
test: test-unit test-functional test-lint

test-unit: WHAT = ./pkg/... ./cmd/...
test-unit:
	${DO} "./hack/build/run-tests.sh ${WHAT}"

test-functional:  WHAT = ./tests/...
test-functional:
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

test-functional-ci: build-functest test-functional

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
	./cluster-sync/sync.sh DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}

bazel-generate:
	SYNC_VENDOR=true ${DO_BAZ} "./hack/build/bazel-generate.sh"

bazel-cdi-generate:
	${DO_BAZ} "./hack/build/bazel-generate.sh -- pkg/ tools/ tests/ cmd/"

bazel-build:
	${DO_BAZ} "./hack/build/bazel-build.sh"

bazel-build-images:	bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-build-images.sh"

bazel-push-images: bazel-cdi-generate bazel-build
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-push-images.sh"

push: bazel-push-images

