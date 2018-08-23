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

.PHONY: build build-controller build-importer build-functest-image-init build-functest-image-http  \
		docker docker-controller docker-cloner docker-importer docker-functest-image-init docker-functest-image-http\
		cluster-sync cluster-sync-controller cluster-sync-cloner cluster-sync-importer \
		test test-functional test-unit \
		publish \
		vet \
		format \
		manifests \
		goveralls \
		release-description \
		fmt \
		fmtcheck \
		lint

DOCKER=1
ifeq (${DOCKER}, 1)
DO=./hack/build/in-docker.sh
else
DO=eval
endif
SOURCE_DIRS = pkg tests tools

all: docker fmt

clean:
	${DO} "./hack/build/build-go.sh clean; rm -rf bin/* _out/* manifests/generated/* .coverprofile release-announcement"

build:
	${DO} "./hack/build/build-go.sh clean && ./hack/build/build-cdi-func-test-file-host.sh && ./hack/build/build-go.sh build ${WHAT} && DOCKER_REPO=${DOCKER_REPO} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} ./hack/build/build-manifests.sh ${WHAT} && ./hack/build/build-copy-artifacts.sh ${WHAT}"

build-controller: WHAT = cmd/cdi-controller
build-controller: build
build-importer: WHAT = cmd/cdi-importer
build-importer: build
# Note, the cloner is a bash script and has nothing to build
build-functest-image-init: WHAT = tools/cdi-func-test-file-host-init
build-functest-image-init: build
build-functest-image-http: WHAT = tools/cdi-func-test-file-host-http
build-functest-image-http: build

# WHAT must match go tool style package paths for test targets (e.g. ./path/to/my/package/...)
test: test-unit test-functional

test-unit: WHAT = ./pkg/...
test-unit:
	${DO} "./hack/build/run-tests.sh ${WHAT}"

test-functional:  WHAT = ./tests/...
test-functional:
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

docker: build
	./hack/build/build-docker.sh build ${WHAT}

docker-controller: WHAT = cmd/cdi-controller
docker-controller: docker
docker-importer: WHAT = cmd/cdi-importer
docker-importer: docker
docker-cloner: WHAT = cmd/cdi-cloner
docker-cloner: docker
docker-functest-image-init: WHAT = tools/cdi-func-test-file-host-init
docker-functest-image-init: docker
docker-functest-image-http: WHAT = tools/cdi-func-test-file-host-http
docker-functest-image-http: docker

push: docker
	./hack/build/build-docker.sh push ${WHAT}

push-controller: WHAT = cmd/cdi-controller
push-controller: push
push-importer: WHAT = cmd/cdi-importer
push-importer: push
push-cloner: WHAT = cdm/cdi-cloner
push-cloner: push

publish: docker
	./hack/build/build-docker.sh publish ${WHAT}

vet:
	${DO} "./hack/build/build-go.sh vet ${WHAT}"

format:
	${DO} "./hack/build/format.sh"

manifests:
	${DO} "DOCKER_REPO=${DOCKER_REPO} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} ./hack/build/build-manifests.sh"

goveralls:
	${DO} "TRAVIS_JOB_ID=${TRAVIS_JOB_ID} TRAVIS_PULL_REQUEST=${TRAVIS_PULL_REQUEST} TRAVIS_BRANCH=${TRAVIS_BRANCH} ./hack/build/goveralls.sh"

checkformat:
	## Just run gofmt without attempting to update files
	@gofmt -l -s $(SOURCE_DIRS) | grep ".*\.go"; if [ "$$?" = "0" ]; then exit 1; fi
lint:
	## Run golint on source files
	@$(foreach dir,$(SOURCE_DIRS),echo "loop dir: $(dir)\n" && golint "$(dir)/...";)

release-description:
	./hack/build/release-description.sh ${RELREF} ${PREREF}

cluster-up:
	./cluster/up.sh

cluster-down:
	./cluster/down.sh

cluster-clean:
	./cluster/clean.sh

cluster-sync: cluster-clean build ${WHAT}
	./cluster/sync.sh ${WHAT}

cluster-sync-controller: WHAT = cmd/cdi-controller
cluster-sync-controller: cluster-sync
cluster-sync-importer: WHAT = cmd/cdi-importer
cluster-sync-importer: cluster-sync
cluster-sync-cloner: WHAT = cmd/cdi-cloner
cluster-sync-cloner: cluster-sync
