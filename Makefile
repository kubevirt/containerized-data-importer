REPO_ROOT=$(abspath $(dir $(lastword $(MAKEFILE_LIST))))
BIN_DIR=$(REPO_ROOT)/bin
BIN_TARGET=cnv-copy-controller
PKG_DIR=$(REPO_ROOT)/main
BUILD_DIR=$(REPO_ROOT)/build

# DOCKER TAG VARS
REGISTRY=gcr.io/openshift-gce-devel
CONTROLLER_IMAGE=cnv-copy-controller
DIRTY_HASH=$(shell git describe --always --abbrev=7 --dirty)
VERSION=v1

.PHONY: controller controller_image release push clean
all: clean controller controller_image

# Compile controller binary
controller: $(BIN_DIR)/$(BIN_TARGET)
$(BIN_DIR)/$(BIN_TARGET): $(PKG_DIR)/main.go
	go build -i -o $(BIN_DIR)/$(BIN_TARGET) $(PKG_DIR)/main.go

# build the controller image
controller_image: $(BUILD_DIR)/Dockerfile
	$(eval TEMP_BUILD_DIR=$(BUILD_DIR)/tmp)
	mkdir -p $(TEMP_BUILD_DIR)
	cp $(BIN_DIR)/$(BIN_TARGET) $(TEMP_BUILD_DIR)
	cp $(BUILD_DIR)/Dockerfile $(TEMP_BUILD_DIR)
	docker build -t $(IMAGE) $(TEMP_BUILD_DIR)
	docker tag $(CONTROLLER_IMAGE) $(REGISTRY)/$(CONTROLLER_IMAGE):$(DIRTY_HASH)
	rm -rf $(TEMP_BUILD_DIR)

# push CONTROLLER_IMAGE:$(DIRTY_HASH). Intended to push controller built from non-master / working branch.
push:
	gcloud docker -- push $(REGISTRY)/$(CONTROLLER_IMAGE):$(DIRTY_HASH)
	@echo ""
	@echo "-- Pushed image:"
	@echo ""
	@echo "        $(REGISTRY)/$(CONTROLLER_IMAGE):$(DIRTY_HASH)"
	@echo ""
	@echo "-- Be sure to update chart/values.yaml!"
	@echo ""

# push CONTROLLER_IMAGE:$(VERSION). Intended to release stable image built from master branch.
release:
	git fetch origin
ifneq ($(shell git rev-parse --abbrev-ref HEAD), master)
	$(error Release is intended to be run on master branch. Please checkout master and retry.)
endif
ifneq ($(shell git rev-list HEAD..origin/master --count), 0)
	$(error HEAD is behind origin/master -- $(shell git status -sb --porcelain))
endif
ifneq ($(shell git rev-list origin/master..HEAD --count), 0)
	$(error HEAD is ahead of origin/master --  $(shell git status -sb --porcelain))
endif
	docker tag $(IMAGE) $(REGISTRY)/$(CONTROLLER_IMAGE):$(VERSION)
	gcloud docker -- push $(REGISTRY)/$(CONTROLLER_IMAGE)

clean:
	rm -rf $(BIN_DIR)/*
	rm -rf $(BUILD_DIR)/tmp
