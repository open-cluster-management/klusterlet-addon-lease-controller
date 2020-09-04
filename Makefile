###############################################################################
# Copyright (c) 2020 Red Hat, Inc.
###############################################################################

SHELL := /bin/bash

export OPERATOR_SDK_VERSION=$(shell operator-sdk version | cut -d '"'  -f 2)

export BINDATA_TEMP_DIR := $(shell mktemp -d)

export GIT_COMMIT      = $(shell git rev-parse --short HEAD)
export GIT_REMOTE_URL  = $(shell git config --get remote.origin.url)
export GITHUB_USER    := $(shell echo $(GITHUB_USER) | sed 's/@/%40/g')
export GITHUB_TOKEN   ?=

export ARCH       ?= $(shell uname -m)
export ARCH_TYPE   = $(if $(patsubst x86_64,,$(ARCH)),$(ARCH),amd64)
export BUILD_DATE  = $(shell date +%m/%d@%H:%M:%S)
export VCS_REF     = $(if $(shell git status --porcelain),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT))

export CGO_ENABLED  = 0
export GO111MODULE := on
export GOOS         = $(shell go env GOOS)
export GOARCH       = $(ARCH_TYPE)
export GOPACKAGES   = $(shell go list ./... | grep -v /vendor | grep -v /internal | grep -v /build | grep -v /test)

export PROJECT_DIR            = $(shell 'pwd')
export BUILD_DIR              = $(PROJECT_DIR)/build
export COMPONENT_SCRIPTS_PATH = $(BUILD_DIR)

export COMPONENT_NAME ?= $(shell cat ./COMPONENT_NAME 2> /dev/null)
export COMPONENT_VERSION ?= $(shell cat ./COMPONENT_VERSION 2> /dev/null)

## WARNING: OPERATOR-SDK - IMAGE_DESCRIPTION & DOCKER_BUILD_OPTS MUST NOT CONTAIN ANY SPACES
export IMAGE_DESCRIPTION ?= RCM_Controller
export DOCKER_FILE        = $(BUILD_DIR)/Dockerfile
export DOCKER_REGISTRY   ?= quay.io
export DOCKER_NAMESPACE  ?= open-cluster-management
export DOCKER_IMAGE      ?= $(COMPONENT_NAME)
export DOCKER_BUILD_TAG  ?= latest
export DOCKER_TAG        ?= $(shell whoami)
export DOCKER_BUILD_OPTS  = --build-arg VCS_REF=$(VCS_REF) \
	--build-arg VCS_URL=$(GIT_REMOTE_URL) \
	--build-arg IMAGE_NAME=$(DOCKER_IMAGE) \
	--build-arg IMAGE_DESCRIPTION=$(IMAGE_DESCRIPTION) \
	--build-arg ARCH_TYPE=$(ARCH_TYPE) \
	--build-arg REMOTE_SOURCE=. \
	--build-arg REMOTE_SOURCE_DIR=/remote-source \
	--build-arg GITHUB_TOKEN=$(GITHUB_TOKEN)

BEFORE_SCRIPT := $(shell build/before-make.sh)

USE_VENDORIZED_BUILD_HARNESS ?=

ifndef USE_VENDORIZED_BUILD_HARNESS
-include $(shell curl -s -H 'Authorization: token ${GITHUB_TOKEN}' -H 'Accept: application/vnd.github.v4.raw' -L https://api.github.com/repos/open-cluster-management/build-harness-extensions/contents/templates/Makefile.build-harness-bootstrap -o .build-harness-bootstrap; echo .build-harness-bootstrap)
else
-include vbh/.build-harness-vendorized
endif

# Only use git commands if it exists
ifdef GIT
GIT_COMMIT      = $(shell git rev-parse --short HEAD)
GIT_REMOTE_URL  = $(shell git config --get remote.origin.url)
VCS_REF     = $(if $(shell git status --porcelain),$(GIT_COMMIT)-$(BUILD_DATE),$(GIT_COMMIT))
endif

# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:trivialVersions=true"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manager


.PHONY: deps
## Download all project dependencies
deps: init component/init

.PHONY: check
## Runs a set of required checks
check: copyright-check #go-bindata-check

.PHONY: go-bindata
go-bindata:
	@if which go-bindata > /dev/null; then \
		echo "##### Updating go-bindata..."; \
		cd $(mktemp -d) && GOSUMDB=off go get -u github.com/go-bindata/go-bindata/...; \
	fi
	@go-bindata --version
	go-bindata -nometadata -pkg bindata -o pkg/bindata/bindata_generated.go -prefix resources/  resources/... COMPONENT_VERSION COMPONENT_NAME

.PHONY: gobindata-check
go-bindata-check:
	@if which go-bindata > /dev/null; then \
		echo "##### Updating go-bindata..."; \
		cd $(mktemp -d) && GOSUMDB=off go get -u github.com/go-bindata/go-bindata/...; \
	fi
	@go-bindata --version
	@echo "##### go-bindata-check ####"
	@go-bindata -nometadata -pkg bindata -o $(BINDATA_TEMP_DIR)/bindata_generated.go -prefix resources/  resources/... COMPONENT_VERSION COMPONENT_NAME; \
	diff $(BINDATA_TEMP_DIR)/bindata_generated.go pkg/bindata/bindata_generated.go > go-bindata.diff; \
	if [ $$? != 0 ]; then \
	  echo "#### Difference detected and saved in go-bindata.diff, run 'make go-bindata' to regenerate the bindata_generated.go"; \
	  cat go-bindata.diff; \
	  exit 1; \
	fi
	@echo "##### go-bindata-check #### Success"

.PHONY: copyright-check
copyright-check:
	./build/copyright-check.sh $(TRAVIS_BRANCH)

.PHONY: functional-test
functional-test:
	ginkgo -tags functional -v --slowSpecThreshold=30 test/functional -- -v=1

.PHONY: functional-test-full
functional-test-full: go-bindata
	$(SELF) component/build COMPONENT_TAG_EXTENSION=-coverage COMPONENT_BUILD_COMMAND=$(PWD)/build/build-coverage.sh 
	$(SELF) component/test/functional COMPONENT_TAG_EXTENSION=-coverage

.PHONY: kind-cluster-setup
kind-cluster-setup: 

# Run tests
test: generate fmt vet manifests component/test/unit
	

# Build manager binary
manager: generate fmt vet
	go build -o bin/manager main.go

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate go-bindata fmt vet manifests
	go run ./main.go -lease-name addon-lease -lease-namespace open-cluster-management-self-import -hub-kubeconfig-secret appmgr-hub-kubeconfig

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build lease/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build lease/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build lease/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=lease/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet ./...

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the docker image
docker-build: test
	docker build . -t ${IMG}

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.3.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	@{ \
	set -e ;\
	KUSTOMIZE_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$KUSTOMIZE_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/kustomize/kustomize/v3@v3.5.4 ;\
	rm -rf $$KUSTOMIZE_GEN_TMP_DIR ;\
	}
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd lease/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build lease/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: build
## Builds controller binary inside of an image
build: component/build
