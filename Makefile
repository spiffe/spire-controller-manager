BINARIES := spire-controller-manager
PLATFORMS ?= linux/amd64,linux/arm64
DIR := ${CURDIR}

# Image URL to use all building/pushing image targets
IMG ?= ghcr.io/spiffe/spire-controller-manager:devel
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.24

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

# Used to force some rules to run every time
.PHONY: FORCE
FORCE: ;

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ OS/ARCH detection

os1=$(shell uname -s)
os2=
ifeq ($(os1),Darwin)
os1=darwin
os2=osx
else ifeq ($(os1),Linux)
os1=linux
os2=linux
else
$(error unsupported OS: $(os1))
endif

arch1=$(shell uname -m)
ifeq ($(arch1),x86_64)
arch2=amd64
else ifeq ($(arch1),aarch64)
arch2=arm64
else ifeq ($(arch1),arm64)
arch2=arm64
else
$(error unsupported ARCH: $(arch1))
endif

##@ Vars

build_dir := $(DIR)/.build/$(os1)-$(arch1)

golangci_lint_version = v1.50.1
golangci_lint_dir = $(build_dir)/golangci_lint/$(golangci_lint_version)
golangci_lint_bin = $(golangci_lint_dir)/golangci-lint
golangci_lint_cache = $(golangci_lint_dir)/cache

##@ Install toolchain

install-golangci-lint: $(golangci_lint_bin)

$(golangci_lint_bin):
	@echo "Installing golangci-lint $(golangci_lint_version)..."
	$(E)rm -rf $(dir $(golangci_lint_dir))
	$(E)mkdir -p $(golangci_lint_dir)
	$(E)mkdir -p $(golangci_lint_cache)
	$(E)GOBIN=$(golangci_lint_dir) $(go_path) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(golangci_lint_version)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)" go test ./... -coverprofile cover.out

##@ Code cleanliness

.PHONY: lint lint-code
lint: lint-code

lint-code: $(golangci_lint_bin)
	$(E)PATH="$(PATH):$(go_bin_dir)" $(golangci_lint_bin) run ./...

##@ Build

.PHONY: build
build: $(addprefix bin,/$(BINARIES)) ## Build manager binary.

bin/%: main.go generate fmt vet FORCE
	go build -o $@ $<

.PHONY: run
run: build ## Run a controller from your host.
	bin/spire-controller-manager

.PHONY: container-builder
container-builder: ## Create a buildx node to create crossplatform images.
	docker buildx create --platform $(PLATFORMS) --name container-builder --node container-builder0 --use

.PHONY: docker-build
docker-build: $(addsuffix -image.tar,$(BINARIES)) ## Build docker image with the manager.

spire-controller-manager-image.tar: Dockerfile FORCE | container-builder
	docker buildx build \
		--platform $(PLATFORMS) \
		--target spire-controller-manager \
		-o type=oci,dest=$@ \
	    .

.PHONY: load-images
load-images: $(addsuffix -image.tar,$(BINARIES)) ## Load the image for your current PLATFORM into docker from the cross-platform oci tar.
	./.github/workflows/scripts/load-oci-archives.sh

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	./.github/workflows/scripts/push-images.sh

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

CONTROLLER_GEN = $(shell pwd)/bin/controller-gen
.PHONY: controller-gen
controller-gen: ## Download controller-gen locally if necessary.
	$(call go-get-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.1)

KUSTOMIZE = $(shell pwd)/bin/kustomize
.PHONY: kustomize
kustomize: ## Download kustomize locally if necessary.
	$(call go-get-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v3@v3.8.7)

ENVTEST = $(shell pwd)/bin/setup-envtest
.PHONY: envtest
envtest: ## Download envtest-setup locally if necessary.
	$(call go-get-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest@latest)

# go-get-tool will 'go install' any package $2 and install it to $1.
PROJECT_DIR := $(shell dirname $(abspath $(lastword $(MAKEFILE_LIST))))
define go-get-tool
@[ -f $(1) ] || { \
set -e ;\
GOBIN=$(PROJECT_DIR)/bin go install $(2) ;\
}
endef
