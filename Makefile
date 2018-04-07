REPO_ROOT=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))

# Basenames
CONTROLLER=controller
IMPORTER=importer
F_TEST=datastream-test

# Binary Path
BIN_DIR=$(REPO_ROOT)/bin
CONTROLLER_BIN=$(BIN_DIR)/import-controller
IMPORTER_BIN=$(BIN_DIR)/$(IMPORTER)
F_TEST_BIN=$(BIN_DIR)/$(F_TEST)

# Source dirs
CMD_DIR=$(REPO_ROOT)/cmd
CONTROLLER_CMD=$(CMD_DIR)/$(CONTROLLER)
IMPORTER_CMD=$(CMD_DIR)/$(IMPORTER)
F_TEST_DIR=$(REPO_ROOT)/test/datastream
F_IMG_DIR=$(REPO_ROOT)/test/images/tinyCore.iso

# Build Dirs
BUILD_DIR=$(REPO_ROOT)/build
CONTROLLER_BUILD=$(BUILD_DIR)/$(CONTROLLER)
IMPORTER_BUILD=$(BUILD_DIR)/$(IMPORTER)

# DOCKER TAG VARS
REGISTRY=jcoperh
GIT_USER=$(shell git config --get user.email | sed 's/@.*//')
TAG="$(GIT_USER)-latest"
VERSION=v1

.PHONY: controller importer controller-bin importer-bin controller-image importer-image push-controller push-importer clean test
all: clean controller importer test
controller: controller-bin controller-image
importer: importer-bin importer-image
push: push-importer push-controller
test: functional-test unit-test
functional-test: func-test-bin func-test-image func-test-run

GOOS?=linux
ARCH?=amd64
CGO_ENABLED=0
LDFLAGS='-extldflags "-static"'

# Compile controller binary
controller-bin:
	@echo '********'
	@echo 'Compiling controller binary'
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=$(CGO_ENABLED) go build -a -ldflags $(LDFLAGS) -o $(CONTROLLER_BIN) $(CONTROLLER_CMD)/*.go

# Compile importer binary
importer-bin:
	@echo '********'
	@echo 'Compiling importer binary'
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=$(CGO_ENABLED) go build -a -ldflags $(LDFLAGS) -o $(IMPORTER_BIN) $(IMPORTER_CMD)/*.go


# Compile datastream functional test binary
func-test-bin:
	@echo '********'
	@echo 'Compiling functional test binary'
	-rm -f $(F_TEST_BIN)
	GOOS=$(GOOS) GOARCH=$(ARCH) CGO_ENABLED=$(CGO_ENABLED) go test -a -c -ldflags $(LDFLAGS) -o $(F_TEST_BIN) $(F_TEST_DIR)/*.go

# build the controller image
controller-image: $(CONTROLLER_BUILD)/Dockerfile
	@echo '********'
	@echo 'Building controller image'
	$(eval TEMP_BUILD_DIR=$(CONTROLLER_BUILD)/tmp)
	mkdir -p $(TEMP_BUILD_DIR)
	cp $(CONTROLLER_BIN) $(TEMP_BUILD_DIR)
	cp $(CONTROLLER_BUILD)/Dockerfile $(TEMP_BUILD_DIR)
	docker build -t $(CONTROLLER) $(TEMP_BUILD_DIR)
	docker tag $(CONTROLLER) $(REGISTRY)/$(CONTROLLER):$(TAG)
	-rm -rf $(TEMP_BUILD_DIR)

# build the importer image
importer-image: $(IMPORTER_BUILD)/Dockerfile
	@echo '********'
	@echo 'Building importer image'
	$(eval TEMP_BUILD_DIR=$(IMPORTER_BUILD)/tmp)
	mkdir -p $(TEMP_BUILD_DIR)
	cp $(IMPORTER_BIN) $(TEMP_BUILD_DIR)
	cp $(IMPORTER_BUILD)/Dockerfile $(TEMP_BUILD_DIR)
	docker build --build-arg entrypoint=$(IMPORTER) --build-arg runArgs='-alsologtostderr' -t $(IMPORTER) $(TEMP_BUILD_DIR)
	docker tag $(IMPORTER) $(REGISTRY)/$(IMPORTER):$(TAG)
	-rm -rf $(TEMP_BUILD_DIR)

# build the functional test image.  The importer image is used to provide consistency between test
# and run environments.
func-test-image: $(IMPORTER_BUILD)/Dockerfile
	@echo '********'
	@echo 'Building functional test image'
	$(eval TEMP_BUILD_DIR=$(IMPORTER_BUILD)/tmp)
	mkdir -p $(TEMP_BUILD_DIR)
	cp $(F_TEST_BIN) $(TEMP_BUILD_DIR)
	cp $(F_IMG_DIR) $(TEMP_BUILD_DIR)
	cp $(IMPORTER_BUILD)/Dockerfile $(TEMP_BUILD_DIR)
	docker build --build-arg entrypoint=$(F_TEST) --build-arg runArgs='-ginkgo.v' --build-arg depFile=tinyCore.iso -t $(F_TEST) $(TEMP_BUILD_DIR)
	-rm -rf $(TEMP_BUILD_DIR)


func-test-run:
	@echo '********'
	@echo 'Running functional tests'
	docker ps -qa && \
	docker run --rm $(F_TEST) || echo 'Docker service not detected, skipping functional tests'

push-controller:
	@echo '********'
	@echo 'Pushing controller image'
	docker push $(REGISTRY)/$(CONTROLLER):$(TAG)

push-importer:
	@echo '********'
	@echo 'Pushing importer image'
	docker push $(REGISTRY)/$(IMPORTER):$(TAG)

unit-test:
	@echo '********'
	@echo 'Running unit tests'
	CGO_ENABLED=$(CGO_ENABLED) go test -v ./... -tags=unit


clean:
	@echo '********'
	@echo 'Cleaning build artifacts'
	-rm -rf $(BIN_DIR)/*
	-rm -rf $(CONTROLLER_BUILD)/tmp
	-rm -rf $(IMPORTER_BUILD)/tmp

# push CONTROLLER:$(VERSION). Intended to release stable image built from master branch.
release:
	@echo '********'
	@echo 'Releasing CDI images'
	docker tag $(IMPORTER) $(REGISTRY)/$(IMPORTER):latest
	docker push $(REGISTRY)/$(IMPORTER):latest
	docker tag $(CONTROLLER) $(REGISTRY)/$(CONTROLLER):latest
	docker push $(REGISTRY)/$(CONTROLLER):latest


my-golden-pvc.yaml: manifests/golden-pvc.yaml
	sed "s,endpoint:.*,endpoint: \"$(URI)\"," $< > $@

.PHONY: my-golden-pvc.yaml
