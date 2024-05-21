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
		coverage \
		release-description \
		bazel-generate bazel-build bazel-build-images bazel-push-images \
		fossa \
		lint-metrics	\
		help

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
# x86_64 aarch64 crossbuild-aarch64
BUILD_ARCH?=x86_64

.DEFAULT_GOAL := help
help: ## Print this message and exit
	@awk 'BEGIN {                                                          \
		FS = ":.*##";                                                      \
		printf "USAGE\n\n    make \033[36m<target>\033[0m\n\nTARGETS\n"    \
	}                                                                      \
	/^[a-zA-Z_0-9-]+:.*?##/ {                                              \
		# Print targets and descriptions                                   \
		printf "    \033[36m%-25s\033[0m%s\n", $$1, $$2                    \
	}                                                                      \
	' $(MAKEFILE_LIST)

all: manifests bazel-build-images

clean:
	${DO_BAZ} "./hack/build/build-go.sh clean; rm -rf bin/* _out/* manifests/generated/* .coverprofile release-announcement"
	${DO_BAZ} bazel clean --expunge

update-codegen:
	${DO_BAZ} "./hack/update-codegen.sh"

generate: update-codegen bazel-generate generate-doc

generate-verify: generate bootstrap-ginkgo
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

gomod-update:
	${DO_BAZ} "./hack/build/dep-update.sh"

deps-update: gomod-update bazel-generate

deps-verify: deps-update
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

rpm-deps:
	${DO_BAZ} "CUSTOM_REPO=${CUSTOM_REPO} ./hack/build/rpm-deps.sh"

apidocs:
	${DO_BAZ} "./hack/update-codegen.sh && ./hack/gen-swagger-doc/gen-swagger-docs.sh v1beta1 html"

build-functest:
	${DO_BAZ} ./hack/build/build-functest.sh

# WHAT must match go tool style package paths for test targets (e.g. ./path/to/my/package/...)
test: test-unit test-functional test-lint

test-unit: WHAT = ./pkg/... ./cmd/...
test-unit:
	${DO_BAZ} "ACK_GINKGO_DEPRECATIONS=${ACK_GINKGO_DEPRECATIONS} ./hack/build/run-unit-tests.sh ${WHAT}"

test-functional:  WHAT = ./tests/...
test-functional: build-functest
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

# test-lint runs gofmt and golint tests against src files
test-lint: lint-metrics
	${DO_BAZ} "./hack/build/run-lint-checks.sh"
	"./hack/ci/language.sh"

docker-registry-cleanup:
	./hack/build/cleanup_docker.sh

publish: manifests push

vet:
	${DO_BAZ} "./hack/build/build-go.sh vet ${WHAT}"

vulncheck:
	${DO_BAZ} ./hack/build/run-vulncheck.sh

format:
	${DO_BAZ} "./hack/build/format.sh"

manifests:
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} CR_NAME=${CR_NAME} CDI_NAMESPACE=${CDI_NAMESPACE} ./hack/build/build-manifests.sh"

goveralls: test-unit
	${DO_BAZ} "COVERALLS_TOKEN_FILE=${COVERALLS_TOKEN_FILE} COVERALLS_TOKEN=${COVERALLS_TOKEN} CI_NAME=prow CI_BRANCH=${PULL_BASE_REF} CI_PR_NUMBER=${PULL_NUMBER} GIT_ID=${PULL_PULL_SHA} PROW_JOB_ID=${PROW_JOB_ID} ./hack/build/goveralls.sh"

coverage: test-unit
	./hack/build/coverage.sh

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
	./cluster-sync/sync.sh CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} PULL_POLICY=${PULL_POLICY} CDI_NAMESPACE=${CDI_NAMESPACE}

cluster-sync-test-infra: cluster-clean-test-infra
	CDI_SYNC="test-infra" ./cluster-sync/sync.sh CDI_AVAILABLE_TIMEOUT=${CDI_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} PULL_POLICY=${PULL_POLICY} CDI_NAMESPACE=${CDI_NAMESPACE}

cluster-sync: cluster-sync-cdi cluster-sync-test-infra

bazel-generate:
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/bazel-generate.sh -- staging/src pkg/ tools/ tests/ cmd/ vendor/"

bazel-cdi-generate:
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/bazel-generate.sh -- staging/src pkg/ tools/ tests/ cmd/"

bazel-build:
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/bazel-build.sh"

gosec:
	${DO_BAZ} "GOSEC=${GOSEC} ./hack/build/gosec.sh"

bazel-build-images:	bazel-cdi-generate bazel-build
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/bazel-build-images.sh"

bazel-push-images: bazel-cdi-generate bazel-build
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} DOCKER_CA_CERT_FILE=${DOCKER_CA_CERT_FILE} ./hack/build/bazel-push-images.sh"

push: bazel-push-images

builder-push:
	./hack/build/bazel-build-builder.sh

openshift-ci-image-push:
	./hack/build/osci-image-builder.sh

generate-doc: build-docgen
	_out/tools/metricsdocs/metricsdocs > doc/metrics.md

bootstrap-ginkgo:
	${DO_BAZ} ./hack/build/bootstrap-ginkgo.sh

build-docgen:
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/bazel-build-metricsdocs.sh"

fossa:
	${DO_BAZ} "FOSSA_TOKEN_FILE=${FOSSA_TOKEN_FILE} PULL_BASE_REF=${PULL_BASE_REF} CI=${CI} ./hack/fossa.sh"

lint-metrics:
	./hack/ci/prom_metric_linter.sh --operator-name="kubevirt" --sub-operator-name="cdi"

