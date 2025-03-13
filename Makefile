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
# x86_64 aarch64 crossbuild-aarch64 s390x crossbuild-s390x
BUILD_ARCH?=$(shell uname -m)

##@ General
.DEFAULT_GOAL := help
help: ## Print this message and exit
	@awk 'BEGIN {                                                          \
		FS = ":.*##";                                                      \
		printf "USAGE\n\n    make \033[36m<target>\033[0m\n\nTARGETS\n"    \
	}                                                                      \
	/^##@/ {                                                               \
		# Print section titles                                             \
		printf "\n  \033[1m%s\033[0m\n", substr($$0, 5)                    \
	}                                                                      \
	/^[a-zA-Z_0-9-]+:.*?##/ {                                              \
		# Print targets and descriptions                                   \
		printf "    \033[36m%-25s\033[0m%s\n", $$1, $$2                    \
	}                                                                      \
	' $(MAKEFILE_LIST)
	@echo
	@echo $(shell printf "\n  \033[1m%s\033[0m\n" "Environment variables")
	@echo "    KUBEVIRTCI_RUNTIME          The runtime to use for the cluster. Default is 'docker' if installed, otherwise 'podman'."
	@echo "    DOCKER_PREFIX               Set repo globally for image and manifest creation. Default is 'quay.io/kubevirt'."
	@echo "    CONTROLLER_IMAGE_NAME       The name of the controller image. Default is 'cdi-controller'."
	@echo "    IMPORTER_IMAGE_NAME         The name of the importer image. Default is 'cdi-importer'."
	@echo "    CLONER_IMAGE_NAME           The name of the cloner image. Default is 'cdi-cloner'."
	@echo "    APISERVER_IMAGE_NAME        The name of the apiserver image. Default is 'cdi-apiserver'."
	@echo "    UPLOADPROXY_IMAGE_NAME      The name of the uploadproxy image. Default is 'cdi-uploadproxy'."
	@echo "    UPLOADSERVER_IMAGE_NAME     The name of the upload server image. Default is 'cdi-uploadserver'."
	@echo "    OPERATOR_IMAGE_NAME         The name of the operator image. Default is 'cdi-operator'."
	@echo "    DOCKER_TAG                  Set global version tags for image and manifest creation. Default is 'latest'."
	@echo "    VERBOSITY                   Set global log level verbosity. Default is '1'."
	@echo "    PULL_POLICY                 Set global CDI pull policy. Default is 'IfNotPresent'."
	@echo "    CR_NAME                     Name of the CDI custom resource. Default is 'cdi'."
	@echo "    CDI_NAMESPACE               Namespace for CDI resources. Default is 'cdi'."
	@echo "    CSV_VERSION                 Version of CSV generated files. Default is '0.0.0'."
	@echo "    QUAY_REPOSITORY             Quay repository. Default is 'cdi-operatorhub'."
	@echo "    QUAY_NAMESPACE              Quay namespace. Default is 'kubevirt'."
	@echo "    TEST_ARGS                   List of additional ginkgo flags to be passed to functional tests. The string "--test-args=" must prefix the variable value."
	@echo "    WHAT                        Path to the package to test. Default is './pkg/... ./cmd/...' for unit tests and './test/...' for functional tests."
	@echo "    RELREF                      Required by release-description. Must be a commit or tag. Should be newer than $$PREREF."
	@echo "    PREREF                      Required by release-description. Must also be a commit or tag. Should be older than $$RELREF."

all: manifests bazel-build-images ## Clean up previous build artifacts, compile all CDI packages and build containers

clean: ## Clean up previous build artifacts
	${DO_BAZ} "./hack/build/build-go.sh clean; rm -rf bin/* _out/* manifests/generated/* .coverprofile release-announcement"
	${DO_BAZ} bazel clean --expunge

##@ Code generation
update-codegen: ## Re-create generated code
	${DO_BAZ} "./hack/update-codegen.sh"

generate: update-codegen bazel-generate generate-doc ## Re-create all generated files

bootstrap-ginkgo: ## Generate Ginkigo testing boilerplate. See `ginkgo bootstrap --help`.
	${DO_BAZ} ./hack/build/bootstrap-ginkgo.sh

generate-verify: generate bootstrap-ginkgo ## Verify the generated files are up to date
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

gomod-update: ## Update vendored Go code in vendor/ subdirectory.
	${DO_BAZ} "./hack/build/dep-update.sh"

apidocs: ## Generate client-go code (same as 'make generate') and swagger docs
	${DO_BAZ} "./hack/update-codegen.sh && ./hack/gen-swagger-doc/gen-swagger-docs.sh v1beta1 html"

##@ Dependency management
deps-update: gomod-update bazel-generate ## Runs 'go mod tidy' and 'go mod vendor'

deps-verify: deps-update ## Verify dependencies are up to date
	git difftool -y --trust-exit-code --extcmd=./hack/diff-csv.sh

rpm-deps: ## Update RPM dependencies
	${DO_BAZ} "CUSTOM_REPO=${CUSTOM_REPO} ./hack/build/rpm-deps.sh"

##@ Testing
build-functest: ## Build the functional tests (content of tests/ subdirectory)
	${DO_BAZ} ./hack/build/build-ginkgo.sh
	${DO_BAZ} ./hack/build/build-functest.sh

test: test-unit test-functional test-lint ## execute all tests (_NOTE:_ 'WHAT' is expected to match the go cli pattern for paths e.g. './pkg/...'.  This differs slightly from rest of the 'make' targets)

test-unit: WHAT = ./pkg/... ./cmd/...
test-unit: ## Run unit tests.
	${DO} "ACK_GINKGO_DEPRECATIONS=${ACK_GINKGO_DEPRECATIONS} ./hack/build/run-unit-tests.sh ${WHAT}"

test-functional: WHAT = ./tests/...
test-functional: build-functest ## Run functional tests (in tests/ subdirectory).
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

goveralls: test-unit ## Run code coverage tracking system and upload it to coveralls
	${DO_BAZ} "COVERALLS_TOKEN_FILE=${COVERALLS_TOKEN_FILE} COVERALLS_TOKEN=${COVERALLS_TOKEN} CI_NAME=prow CI_BRANCH=${PULL_BASE_REF} CI_PR_NUMBER=${PULL_NUMBER} GIT_ID=${PULL_PULL_SHA} PROW_JOB_ID=${PROW_JOB_ID} ./hack/build/goveralls.sh"

coverage: test-unit ## Run code coverage report locally.
	./hack/build/coverage.sh

##@ Image management
docker-registry-cleanup: ## Clean up all cached images from docker registry. Accepts [make variables](#make-variables) DOCKER_PREFIX. Removes all images of the specified repository. If not specified removes localhost repository of current cluster instance.
	./hack/build/cleanup_docker.sh

publish: manifests push ## Generate a cdi-controller and operator manifests and push the built container images to the registry defined in DOCKER_PREFIX

manifests: ## Generate a cdi-controller and operator manifests in '_out/manifests/'.  Accepts [make variables]\(#make-variables\) DOCKER_TAG, DOCKER_PREFIX, VERBOSITY, PULL_POLICY, CSV_VERSION, QUAY_REPOSITORY, QUAY_NAMESPACE
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} CR_NAME=${CR_NAME} CDI_NAMESPACE=${CDI_NAMESPACE} ./hack/build/build-manifests.sh"

release-description: ## Generate a release announcement detailing changes between 2 commits (typically tags).  Expects 'RELREF' and 'PREREF' to be set
	./hack/build/release-description.sh ${RELREF} ${PREREF}

builder-push: ## Build and push the builder container image, declared in docker/builder/Dockerfile.
	./hack/build/bazel-build-builder.sh

openshift-ci-image-push: ## Build and push the OpenShift CI build+test container image, declared in hack/ci/Dockerfile.ci
	./hack/build/osci-image-builder.sh

##@ Local cluster management
cluster-up: ## Start a default Kubernetes or Open Shift cluster. set KUBEVIRT_PROVIDER environment variable to either 'k8s-1.18' or 'os-3.11.0' to select the type of cluster. set KUBEVIRT_NUM_NODES to something higher than 1 to have more than one node.
	./cluster-up/up.sh

cluster-down: ## Stop the cluster, doing a make cluster-down && make cluster-up will basically restart the cluster into an empty fresh state.
	./cluster-up/down.sh

cluster-down-purge: docker-registry-cleanup cluster-down ## Cluster-down and clean up all cached images from docker registry. See docker-registry-cleanup target help.

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

cluster-sync: cluster-sync-cdi cluster-sync-test-infra ## Build the controller/importer/cloner, and push it into a running cluster. The cluster must be up before running a cluster sync. Also generates a manifest and applies it to the running cluster after pushing the images to it.

##@ Bazel
bazel-generate: ## Generate BUILD files for Bazel.
	${DO_BAZ} "./hack/build/bazel-generate.sh -- staging/src pkg/ tools/ tests/ cmd/ vendor/"

bazel-cdi-generate:
	${DO_BAZ} "./hack/build/bazel-generate.sh -- staging/src pkg/ tools/ tests/ cmd/"

bazel-build: ## Build all Go binaries.
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/multi-arch.sh build"

bazel-build-images:	bazel-cdi-generate bazel-build ## Build all the container images used (for both CDI and functional tests)
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} ./hack/build/multi-arch.sh build-images"

bazel-push-images: bazel-cdi-generate bazel-build ## Push the built container images to the registry defined in DOCKER_PREFIX
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} DOCKER_CA_CERT_FILE=${DOCKER_CA_CERT_FILE} ./hack/build/multi-arch.sh push-images"
	BUILD_ARCH=${BUILD_ARCH} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} hack/build/push-container-manifest.sh

push: bazel-push-images ## Same as bazel-push-images

##@ Documentation
build-docgen: ## Build documentation generator
	${DO_BAZ} "BUILD_ARCH=${BUILD_ARCH} ./hack/build/bazel-build-metricsdocs.sh"

generate-doc: build-docgen ## Generate documentation
	_out/tools/metricsdocs/metricsdocs > doc/metrics.md

##@ Linting and code scanning
fossa: ## Run FOSSA security code scanning
	${DO_BAZ} "FOSSA_TOKEN_FILE=${FOSSA_TOKEN_FILE} PULL_BASE_REF=${PULL_BASE_REF} CI=${CI} ./hack/fossa.sh"

lint-metrics: ## Run metrics name linter
	./hack/ci/prom_metric_linter.sh --operator-name="kubevirt" --sub-operator-name="cdi"

test-lint: lint-metrics ## Run linter on source files
	${DO_BAZ} "./hack/build/run-lint-checks.sh"
	"./hack/ci/language.sh"

vet: ## Lint all CDI packages
	${DO_BAZ} "./hack/build/build-go.sh vet ${WHAT}"

vulncheck: ## Scan Go dependencies for known vulnerabilities.
	${DO_BAZ} ./hack/build/run-vulncheck.sh

format: ## Format shell and go source files."
	${DO_BAZ} "./hack/build/format.sh"

.PHONY:	\
	help all clean \
	update-codegen generate bootstrap-ginkgo generate-verify gomod-update apidocs \
	deps-update deps-verify rpm-deps \
	build-functest test test-unit test-functional goveralls coverage \
	docker-registry-cleanup publish manifests release-description builder-push openshift-ci-image-push \
	cluster-up cluster-down cluster-down-purge cluster-sync \
	bazel-generate bazel-build bazel-build-images bazel-push-images push \
	build-docgen generate-doc \
	fossa lint-metrics test-lint vet vulncheck format  \
