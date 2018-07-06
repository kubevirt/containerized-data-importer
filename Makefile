DOCKER=1

.PHONY: build build-controller build-importer \
		docker docker-controller docker-cloner docker-importer \
		test test-functional test-unit \
		publish \
		vet \
		format

all: clean docker

clean:
	./hack/build/clean.sh

build: # TODO vet
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh build ${WHAT}"
else
	./hack/build/build-go.sh build ${WHAT}
endif

build-controller: WHAT = cmd/controller
build-controller: build
build-importer: WHAT = cmd/importer
build-importer: build

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

docker-controller: WHAT = cdi-controller
docker-controller: docker
docker-importer: WHAT = cdi-importer
docker-importer: docker
docker-cloner: WHAT = cdi-cloner
docker-cloner: docker

publish: docker
	./hack/build/build-docker.sh push ${WHAT}

vet:
ifeq (${DOCKER}, 1)
	./hack/build/in-docker "./hack/build/build-go.sh vet ${WHAT}"
else
	./hack/build/build-go.sh vet ${WHAT}
endif

format:
ifeq (${DOCKER}, 1)
	.hack/build/in-docker "./hack/build/format.sh"
else
	./hack/build/format.sh
endif


