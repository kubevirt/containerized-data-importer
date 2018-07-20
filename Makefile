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

DOCKER=1

.PHONY: build build-controller build-importer \
		docker docker-controller docker-cloner docker-importer \
		test test-functional test-unit \
		publish \
		vet \
		format

all: docker

clean:
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh clean && rm -rf bin/* _out/*"
else
	"./hack/build/build-go.sh clean && rm -rf bin/* _out/*"
endif

build:
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh clean && ./hack/build/build-go.sh build ${WHAT} && ./hack/build/build-copy-artifacts.sh ${WHAT}"
else
	./hack/build/build-go.sh clean && ./hack/build/build-go.sh build ${WHAT} && ./hack/build/build-copy-artifacts.sh ${WHAT}
endif

build-controller: WHAT = cmd/cdi-controller
build-controller: build
build-importer: WHAT = cmd/cdi-importer
build-importer: build
# Note, the cloner is a bash script and has nothing to build

test:
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh test ${WHAT}"
else
	./hack/build/build-go.sh test ${WHAT}
endif

test-unit: WHAT = pkg/
test-unit: test
test-functional: WHAT = test/
test-functional: test

docker: build
	./hack/build/build-docker.sh build ${WHAT}

docker-controller: WHAT = cmd/cdi-controller
docker-controller: docker
docker-importer: WHAT = cmd/cdi-importer
docker-importer: docker
docker-cloner: WHAT = cmd/cdi-cloner
docker-cloner: docker

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
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh vet ${WHAT}"
else
	./hack/build/build-go.sh vet ${WHAT}
endif

format:
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/format.sh"
else
	./hack/build/format.sh
endif


