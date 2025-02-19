CURRENT_DIR=$(shell pwd)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.30.0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

GIT_TAG:=$(if $(GIT_TAG),$(GIT_TAG),$(shell if [ -z "`git status --porcelain`" ]; then git describe --exact-match --tags HEAD 2>/dev/null; fi))

# docker image publishing options
IMAGE_NAMESPACE?=argoproj-labs
IMAGE_NAME?=${IMAGE_NAMESPACE}/argocd-ephemeral-access

ifneq (${GIT_TAG},)
IMAGE_TAG=${GIT_TAG}
else
IMAGE_TAG?=latest
endif

IMG=${IMAGE_NAME}:${IMAGE_TAG}

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

UI_DIR=${CURRENT_DIR}/ui

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=controller-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

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
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test $$(go list ./... | grep -v /test | grep -v /cmd | grep -v /api) -coverprofile cover.out

.PHONY: setup-envtest
setup-envtest: envtest ## configure envtest k8s directory.
	$(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN)

# Utilize Kind or modify the e2e tests to load the image locally, enabling compatibility with other vendors.
.PHONY: test-e2e  # Run the e2e tests against a Kind k8s instance that is spun up.
test-e2e:
	go test ./test/e2e/ -v -ginkgo.v

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

##@ Run

.PHONY: run
run: build-go goreman ## Run all ephemeral-access components defined in the Procfile.
	$(GOREMAN) start

.PHONY: run-controller
run-controller: build goreman ## Run a controller from your host.
	$(GOREMAN) start controller

.PHONY: run-backend
run-backend: build goreman ## Run the api backend server.
	$(GOREMAN) start backend

##@ Build

.PHONY: build
build: build-ui build-go ## Build the UI extension and the ephemeral-access binary.

.PHONY: build-go
build-go: manifests generate fmt vet ## Build the ephemeral-access binary.
	go build -o bin/ephemeral-access cmd/main.go

# If you wish to build the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: clean-ui
clean-ui: ## delete the extension.tar file.
	find ${UI_DIR} -type f -name extension.tar -delete

.PHONY: build-ui
build-ui: clean-ui ## build the Argo CD UI extension creating the ui/extension.tar file.
	yarn --cwd ${UI_DIR} install
	yarn --cwd ${UI_DIR} build

.PHONY: openapi-ui
openapi-ui:  build-go goreman  ## Update the OpenAPI spec from the local server.
	@echo "Updating OpenAPI specification..."
	@echo "Starting the backend server"
	goreman start backend &
	sleep 5
	@echo "Downloading OpenAPI spec from dev server"
	yarn --cwd ${UI_DIR} api:download
	killall goreman || true


.PHONY: codegen-ui openapi-ui
codegen-ui: openapi-ui ## Generate the UI API files based on the OpenAPI spec.
	@echo "Generating API files..."
	UI_DIR=${CURRENT_DIR}/ui
	yarn --cwd ${UI_DIR} api:generate
	rm -f ${UI_DIR}/src/gen/schema.yaml

.PHONY: goreleaser-build-local
goreleaser-build-local: goreleaser ## Run goreleaser build locally. Use to validate the goreleaser configuration.
	$(GORELEASER) build --snapshot --clean --single-target --verbose

# PLATFORMS defines the target platforms for the manager image be built to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - be able to use docker buildx. More info: https://docs.docker.com/build/buildx/
# - have enabled BuildKit. More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image to your registry (i.e. if you do not set a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To adequately provide solutions that are compatible with multiple platforms, you should consider using this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: ## Build and push docker image for the manager for cross-platform support.
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name argocd-ephemeral-access-builder
	$(CONTAINER_TOOL) buildx use argocd-ephemeral-access-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm argocd-ephemeral-access-builder
	rm Dockerfile.cross

.PHONY: manifests-release
manifests-release: generate manifests kustomize ## Generate the consolidated install.yaml with the release tag.
	./scripts/manifests-release.sh $(KUSTOMIZE) $(IMAGE_TAG)

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests-release ## Deploy distribution to the K8s cluster specified in ~/.kube/config.
	$(KUBECTL) apply -f install.yaml

.PHONY: undeploy
undeploy: kustomize ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: install-ui-extension
install-ui-extension: build-ui ## build and copy the necessary ui extension files in the Argo CD UI extensions folder.
	mkdir -p /tmp/extensions
	rm -rf /tmp/extensions/resources/extension-ephemeral-access.js
	find ${UI_DIR} -type f -name extension.tar -exec cp {} /tmp/extensions/ephemeral.tar \;
	tar -xf /tmp/extensions/ephemeral.tar --cd /tmp/extensions
	cp test/manifests/samples/extension-0-EPHEMERAL_ACCESS_vars.js /tmp/extensions/resources/extension-ephemeral-access.js/

##@ Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize-$(KUSTOMIZE_VERSION)
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen-$(CONTROLLER_TOOLS_VERSION)
ENVTEST ?= $(LOCALBIN)/setup-envtest-$(ENVTEST_VERSION)
GOLANGCI_LINT = $(LOCALBIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
MOCKERY ?= $(LOCALBIN)/mockery-$(MOCKERY_VERSION)
GORELEASER ?= $(LOCALBIN)/goreleaser-$(GORELEASER_VERSION)
GOREMAN ?= $(LOCALBIN)/goreman-$(GOREMAN_VERSION)

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0
CONTROLLER_TOOLS_VERSION ?= v0.15.0
ENVTEST_VERSION ?= release-0.18
GOLANGCI_LINT_VERSION ?= v1.57.2
MOCKERY_VERSION ?= v2.45.0
GORELEASER_VERSION ?= v2.3.2
GOREMAN_VERSION ?= v0.3.15

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	$(call go-install-tool,$(KUSTOMIZE),sigs.k8s.io/kustomize/kustomize/v5,$(KUSTOMIZE_VERSION))

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	$(call go-install-tool,$(CONTROLLER_GEN),sigs.k8s.io/controller-tools/cmd/controller-gen,$(CONTROLLER_TOOLS_VERSION))

.PHONY: envtest
envtest: $(ENVTEST) ## Download setup-envtest locally if necessary.
$(ENVTEST): $(LOCALBIN)
	$(call go-install-tool,$(ENVTEST),sigs.k8s.io/controller-runtime/tools/setup-envtest,$(ENVTEST_VERSION))

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Download golangci-lint locally if necessary.
$(GOLANGCI_LINT): $(LOCALBIN)
	$(call go-install-tool,$(GOLANGCI_LINT),github.com/golangci/golangci-lint/cmd/golangci-lint,${GOLANGCI_LINT_VERSION})


.PHONY: mockery
mockery: $(MOCKERY) ## Download mockery locally if necessary.
$(MOCKERY): $(LOCALBIN)
	$(call go-install-tool,$(MOCKERY),github.com/vektra/mockery/v2,$(MOCKERY_VERSION))

.PHONY: goreleaser 
goreleaser: $(GORELEASER) ## Download goreleaser locally if necessary.
$(GORELEASER): $(LOCALBIN)
	$(call go-install-tool,$(GORELEASER),github.com/goreleaser/goreleaser/v2,$(GORELEASER_VERSION))

.PHONY: goreman
goreman: $(GOREMAN) ## Download goreman locally if necessary.
$(GOREMAN): $(LOCALBIN)
	$(call go-install-tool,$(GOREMAN),github.com/mattn/goreman,$(GOREMAN_VERSION))

.PHONY: generate-mocks
generate-mocks: mockery ## Generate the mocks for the project as configured in .mockery.yaml
	$(MOCKERY)

# go-install-tool will 'go install' any package with custom target and name of binary, if it doesn't exist
# $1 - target path with name of binary (ideally with version)
# $2 - package url which can be installed
# $3 - specific version of package
define go-install-tool
@[ -f $(1) ] || { \
set -e; \
package=$(2)@$(3) ;\
echo "Downloading $${package}" ;\
GOBIN=$(LOCALBIN) go install $${package} ;\
mv "$$(echo "$(1)" | sed "s/-$(3)$$//")" $(1) ;\
}
endef
