# Copyright 2023 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# If you update this file, please follow
# https://www.thapaliya.com/en/writings/well-documented-makefiles/

# Ensure Make is run with bash shell as some syntax below is bash-specific
SHELL:=/usr/bin/env bash

.DEFAULT_GOAL:=help

#
# Go.
#
GO_VERSION ?= 1.21.5
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

#
# Kubebuilder.
#
export KUBEBUILDER_ENVTEST_KUBERNETES_VERSION ?= 1.26.0
export KUBEBUILDER_CONTROLPLANE_START_TIMEOUT ?= 60s
export KUBEBUILDER_CONTROLPLANE_STOP_TIMEOUT ?= 60s

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

# Enables shell script tracing. Enable by running: TRACE=1 make <target>
TRACE ?= 0

#
# Directories.
#
# Full directory of where the Makefile resides
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
EXP_DIR := exp
BIN_DIR := bin
TEST_DIR := test
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/$(BIN_DIR))
E2E_FRAMEWORK_DIR := $(TEST_DIR)/framework
GO_INSTALL := ./scripts/go_install.sh
OBSERVABILITY_DIR := hack/observability

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/sigs.k8s.io/cluster-api-addon-provider-helm)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
else
	export GOPATH := $(shell go env GOPATH)
endif

#
# Ginkgo configuration.
#
GINKGO_FOCUS ?=
GINKGO_SKIP ?=
GINKGO_NODES ?= 1
GINKGO_TIMEOUT ?= 2h
GINKGO_POLL_PROGRESS_AFTER ?= 60m
GINKGO_POLL_PROGRESS_INTERVAL ?= 5m
E2E_CONF_FILE ?= $(ROOT_DIR)/$(TEST_DIR)/e2e/config/helm.yaml
E2E_CONF_FILE_ENVSUBST := $(ROOT_DIR)/test/e2e/config/helm-envsubst.yaml
SKIP_RESOURCE_CLEANUP ?= false
USE_EXISTING_CLUSTER ?= false
GINKGO_NOCOLOR ?= false

# to set multiple ginkgo skip flags, if any
ifneq ($(strip $(GINKGO_SKIP)),)
_SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
endif

# Helper function to get dependency version from go.mod
get_go_version = $(shell go list -m $1 | awk '{print $$2}')

#
# Binaries.
#
# Note: Need to use abspath so we can invoke these from subdirectories
KUSTOMIZE_VER := v4.5.2
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER))
KUSTOMIZE_PKG := sigs.k8s.io/kustomize/kustomize/v4

MOCKGEN_VER := v0.2.0
MOCKGEN_BIN := mockgen
MOCKGEN := $(TOOLS_BIN_DIR)/$(MOCKGEN_BIN)-$(MOCKGEN_VER)

SETUP_ENVTEST_VER := v0.0.0-20211110210527-619e6b92dab9
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER))
SETUP_ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest

CONTROLLER_GEN_VER := v0.14.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER))
CONTROLLER_GEN_PKG := sigs.k8s.io/controller-tools/cmd/controller-gen

GOTESTSUM_VER := v1.11.0
GOTESTSUM_BIN := gotestsum
GOTESTSUM := $(abspath $(TOOLS_BIN_DIR)/$(GOTESTSUM_BIN)-$(GOTESTSUM_VER))
GOTESTSUM_PKG := gotest.tools/gotestsum

CONVERSION_GEN_VER := v0.29.0
CONVERSION_GEN_BIN := conversion-gen
# We are intentionally using the binary without version suffix, to avoid the version
# in generated files.
CONVERSION_GEN := $(abspath $(TOOLS_BIN_DIR)/$(CONVERSION_GEN_BIN))
CONVERSION_GEN_PKG := k8s.io/code-generator/cmd/conversion-gen

ENVSUBST_BIN := envsubst
ENVSUBST_VER := $(call get_go_version,github.com/drone/envsubst/v2)
ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-$(ENVSUBST_VER))
ENVSUBST_PKG := github.com/drone/envsubst/v2/cmd/envsubst

GO_APIDIFF_VER := v0.7.0
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(abspath $(TOOLS_BIN_DIR)/$(GO_APIDIFF_BIN)-$(GO_APIDIFF_VER))
GO_APIDIFF_PKG := github.com/joelanford/go-apidiff

HADOLINT_VER := v2.12.0
HADOLINT_FAILURE_THRESHOLD = warning

SHELLCHECK_VER := v0.9.0

KPROMO_VER := v4.0.4
KPROMO_BIN := kpromo
KPROMO :=  $(abspath $(TOOLS_BIN_DIR)/$(KPROMO_BIN)-$(KPROMO_VER))
KPROMO_PKG := sigs.k8s.io/promo-tools/v4/cmd/kpromo

RELEASE_NOTES_VER := v0.12.0
RELEASE_NOTES_BIN := release-notes
RELEASE_NOTES := $(TOOLS_BIN_DIR)/$(RELEASE_NOTES_BIN)-$(RELEASE_NOTES_VER)

YQ_VER := v4.25.2
YQ_BIN := yq
YQ :=  $(abspath $(TOOLS_BIN_DIR)/$(YQ_BIN)-$(YQ_VER))
YQ_PKG := github.com/mikefarah/yq/v4

GINKGO_BIN := ginkgo
GINGKO_VER := $(call get_go_version,github.com/onsi/ginkgo/v2)
GINKGO := $(abspath $(TOOLS_BIN_DIR)/$(GINKGO_BIN)-$(GINGKO_VER))
GINKGO_PKG := github.com/onsi/ginkgo/v2/ginkgo

CONVERSION_VERIFIER_BIN := conversion-verifier
CONVERSION_VERIFIER := $(abspath $(TOOLS_BIN_DIR)/$(CONVERSION_VERIFIER_BIN))

OPENAPI_GEN_VER := 5e7f5fd
OPENAPI_GEN_BIN := openapi-gen
# We are intentionally using the binary without version suffix, to avoid the version
# in generated files.
OPENAPI_GEN := $(abspath $(TOOLS_BIN_DIR)/$(OPENAPI_GEN_BIN))
OPENAPI_GEN_PKG := k8s.io/kube-openapi/cmd/openapi-gen

RUNTIME_OPENAPI_GEN_BIN := runtime-openapi-gen
RUNTIME_OPENAPI_GEN := $(abspath $(TOOLS_BIN_DIR)/$(RUNTIME_OPENAPI_GEN_BIN))

TILT_PREPARE_BIN := tilt-prepare
TILT_PREPARE := $(abspath $(TOOLS_BIN_DIR)/$(TILT_PREPARE_BIN))

GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN))

# Define Docker related variables. Releases should modify and double check these vars.
REGISTRY ?= gcr.io/$(shell gcloud config get-value project)
PROD_REGISTRY ?= registry.k8s.io/cluster-api-helm

STAGING_REGISTRY ?= gcr.io/k8s-staging-cluster-api-helm
STAGING_BUCKET ?= artifacts.k8s-staging-cluster-api-helm.appspot.com

# core
IMAGE_NAME ?= cluster-api-helm-controller
CONTROLLER_IMG ?= $(REGISTRY)/$(IMAGE_NAME)

# kind
CAPI_KIND_CLUSTER_NAME ?= capi-test

# It is set by Prow GIT_TAG, a git-based tag of the form vYYYYMMDD-hash, e.g., v20210120-v0.3.10-308-gc61521971

TAG ?= dev
ARCH ?= $(shell go env GOARCH)
ALL_ARCH = amd64 arm arm64 ppc64le s390x

# Allow overriding manifest generation destination directory
MANIFEST_ROOT ?= config
CRD_ROOT ?= $(MANIFEST_ROOT)/crd/bases
WEBHOOK_ROOT ?= $(MANIFEST_ROOT)/webhook
RBAC_ROOT ?= $(MANIFEST_ROOT)/rbac

# Allow overriding the imagePullPolicy
PULL_POLICY ?= Always

# Hosts running SELinux need :z added to volume mounts
SELINUX_ENABLED := $(shell cat /sys/fs/selinux/enforce 2> /dev/null || echo 0)

ifeq ($(SELINUX_ENABLED),1)
  DOCKER_VOL_OPTS?=:z
endif

# Set build time variables including version details
LDFLAGS := $(shell hack/version.sh)

all: test manager

help:  # Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[0-9A-Za-z_-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 } /^\$$\([0-9A-Za-z_-]+\):.*?##/ { gsub("_","-", $$1); printf "  \033[36m%-45s\033[0m %s\n", tolower(substr($$1, 3, length($$1)-7)), $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

## --------------------------------------
## Generate / Manifests
## --------------------------------------

##@ generate:

ALL_GENERATE_MODULES = core kubeadm-bootstrap kubeadm-control-plane docker-infrastructure

.PHONY: generate
generate: ## Run all generate-* targets
	$(MAKE) generate-modules generate-manifests generate-go

.PHONY: generate-manifests
generate-manifests: $(CONTROLLER_GEN) $(KUSTOMIZE) ## Generate manifests e.g. CRD, RBAC etc. for core
	$(CONTROLLER_GEN) \
		paths=./api/... \
		crd:crdVersions=v1 \
		rbac:roleName=manager-role \
		output:crd:dir=$(CRD_ROOT) \
		output:webhook:dir=$(WEBHOOK_ROOT) \
		webhook
	$(CONTROLLER_GEN) \
		paths=./ \
		paths=./controllers/... \
		output:rbac:dir=$(RBAC_ROOT) \
		rbac:roleName=manager-role

.PHONY: generate-go
generate-go: $(CONTROLLER_GEN) $(MOCKGEN) $(KUSTOMIZE) ## Generate manifests e.g. CRD, RBAC etc. for core
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate/boilerplate.generatego.txt" paths="./api/..."
	go generate ./...

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to ensure modules are up to date
	go mod tidy
	cd $(TOOLS_DIR); go mod tidy

DOCKER_TEMPLATES := test/e2e/data/addons-helm

.PHONY: generate-e2e-templates
generate-e2e-templates: $(KUSTOMIZE) $(addprefix generate-e2e-templates-, v1.5 main) ## Generate cluster templates for all versions

.PHONY: generate-e2e-templates-v1.5
generate-e2e-templates-v1.5: $(KUSTOMIZE)
	$(KUSTOMIZE) build $(DOCKER_TEMPLATES)/v1.5/cluster-template --load-restrictor LoadRestrictionsNone > $(DOCKER_TEMPLATES)/v1.5/cluster-template.yaml

.PHONY: generate-e2e-templates-main
generate-e2e-templates-main: $(KUSTOMIZE) ## Generate templates for e2e tests on main branch.
	$(KUSTOMIZE) build $(DOCKER_TEMPLATES)/v1beta1/cluster-template --load-restrictor LoadRestrictionsNone > $(DOCKER_TEMPLATES)/v1beta1/cluster-template.yaml

.PHONY: generate-flavors
generate-flavors: $(KUSTOMIZE)  ## Generate template flavors.
	./hack/gen-flavors.sh

## --------------------------------------
## Lint / Verify
## --------------------------------------

##@ lint and verify:

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint the codebase
	$(GOLANGCI_LINT) run -v $(GOLANGCI_LINT_EXTRA_ARGS)
	# cd $(TEST_DIR); $(GOLANGCI_LINT) run --path-prefix $(TEST_DIR) -v $(GOLANGCI_LINT_EXTRA_ARGS)
	# TODO: add linting for TEST_DIR when e2e tests are set up
	./scripts/ci-lint-dockerfiles.sh $(HADOLINT_VER) $(HADOLINT_FAILURE_THRESHOLD)

.PHONY: lint-dockerfiles
lint-dockerfiles:
	./scripts/ci-lint-dockerfiles.sh $(HADOLINT_VER) $(HADOLINT_FAILURE_THRESHOLD)

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linter
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

APIDIFF_OLD_COMMIT ?= $(shell git rev-parse origin/main)

.PHONY: apidiff
apidiff: $(GO_APIDIFF) ## Check for API differences
	$(GO_APIDIFF) $(APIDIFF_OLD_COMMIT) --print-compatible

ALL_VERIFY_CHECKS = boilerplate shellcheck modules gen

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS)) lint-dockerfiles ## Run all verify-* targets

.PHONY: verify-modules
verify-modules: generate-modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod $(TOOLS_DIR)/go.mod $(TOOLS_DIR)/go.sum $(TEST_DIR)/go.mod $(TEST_DIR)/go.sum); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate  ## Verify go generated files are up to date
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-boilerplate
verify-boilerplate: ## Verify boilerplate text exists in each file
	TRACE=$(TRACE) ./hack/verify-boilerplate.sh

.PHONY: verify-shellcheck
verify-shellcheck: ## Verify shell files
	TRACE=$(TRACE) ./hack/verify-shellcheck.sh $(SHELLCHECK_VER)

.PHONY: verify-container-images
verify-container-images: ## Verify container images
	TRACE=$(TRACE) ./hack/verify-container-images.sh

## --------------------------------------
## Binaries
## --------------------------------------

##@ build:

.PHONY: manager
manager: ## Build the manager binary into the ./bin folder
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/manager .

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites:
	docker pull docker.io/docker/dockerfile:1.4
	docker pull $(GO_CONTAINER_IMAGE)
	docker pull gcr.io/distroless/static:latest

.PHONY: docker-build-all
docker-build-all: $(addprefix docker-build-,$(ALL_ARCH)) ## Build docker images for all architectures

docker-build-%:
	$(MAKE) ARCH=$* docker-build

.PHONY: docker-build
docker-build: docker-pull-prerequisites ## Build the docker image for core controller manager
	DOCKER_BUILDKIT=1 docker build --build-arg builder_image=$(GO_CONTAINER_IMAGE) --build-arg goproxy=$(GOPROXY) --build-arg ARCH=$(ARCH) --build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG)-$(ARCH):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

## --------------------------------------
## Testing
## --------------------------------------

##@ test:

ARTIFACTS ?= ${ROOT_DIR}/_artifacts

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: test
test: $(SETUP_ENVTEST) ## Run unit and integration tests
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test ./... $(TEST_ARGS)

.PHONY: test-verbose
test-verbose: ## Run unit and integration tests with verbose flag
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -v"

.PHONY: test-junit
test-junit: $(SETUP_ENVTEST) $(GOTESTSUM) ## Run unit and integration tests and generate a junit report
	set +o errexit; (KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test -json ./... $(TEST_ARGS); echo $$? > $(ARTIFACTS)/junit.exitcode) | tee $(ARTIFACTS)/junit.stdout
	$(GOTESTSUM) --junitfile $(ARTIFACTS)/junit.xml --raw-command cat $(ARTIFACTS)/junit.stdout
	exit $$(cat $(ARTIFACTS)/junit.exitcode)

.PHONY: test-cover
test-cover: ## Run unit and integration tests and generate a coverage report
	$(MAKE) test TEST_ARGS="$(TEST_ARGS) -coverprofile=out/coverage.out"
	go tool cover -func=out/coverage.out -o out/coverage.txt
	go tool cover -html=out/coverage.out -o out/coverage.html


.PHONY: test-e2e
test-e2e: ## Run `docker-build` and `docker-push` rules then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build docker-push \
	test-e2e-run

.PHONY: test-e2e-local
test-e2e-local: ## Run `docker-build` rule then run e2e tests.
	PULL_POLICY=IfNotPresent MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) docker-build \
	test-e2e-run

.PHONY: test-e2e-run-skip-manifest
test-e2e-run-skip-manifest: $(GINKGO) $(ENVSUBST) generate-e2e-templates ## Run the end-to-end tests without updating the manifest.
	$(ENVSUBST) < $(E2E_CONF_FILE) > $(E2E_CONF_FILE_ENVSUBST) && \
	$(GINKGO) -v --trace -poll-progress-after=$(GINKGO_POLL_PROGRESS_AFTER) \
		-poll-progress-interval=$(GINKGO_POLL_PROGRESS_INTERVAL) --tags=e2e --focus="$(GINKGO_FOCUS)" \
		$(_SKIP_ARGS) --nodes=$(GINKGO_NODES) --timeout=$(GINKGO_TIMEOUT) --no-color=$(GINKGO_NOCOLOR) \
		--output-dir="$(ARTIFACTS)" --junit-report="junit.e2e_suite.1.xml" $(GINKGO_ARGS) $(ROOT_DIR)/$(TEST_DIR)/e2e -- \
	    -e2e.artifacts-folder="$(ARTIFACTS)" \
	    -e2e.config="$(E2E_CONF_FILE_ENVSUBST)" \
	    -e2e.skip-resource-cleanup=$(SKIP_RESOURCE_CLEANUP) -e2e.use-existing-cluster=$(USE_EXISTING_CLUSTER)

.PHONY: test-e2e-run
test-e2e-run: ## Run the end-to-end tests and set controller image and pull policy in the manifest.
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG)-$(ARCH) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml" PULL_POLICY=IfNotPresent
	MANAGER_IMAGE=$(CONTROLLER_IMG)-$(ARCH):$(TAG) \
	$(MAKE) test-e2e-run-skip-manifest

.PHONY: get-e2e-kubeconfig
get-e2e-kubeconfig: ## Get the kubeconfig for the e2e cluster
	@kind get kubeconfig --name caaph-e2e

## --------------------------------------
## Deployment
## --------------------------------------

##@ deployment:

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: generate-manifests $(KUSTOMIZE) ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: generate-manifests $(KUSTOMIZE) ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: generate-manifests $(KUSTOMIZE) ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	$(MAKE) manifest-modification RELEASE_TAG=$(TAG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

## --------------------------------------
## Release
## --------------------------------------

##@ release:

## latest git tag for the commit, e.g., v0.3.10
RELEASE_TAG ?= $(shell git describe --abbrev=0 2>/dev/null)
ifneq (,$(findstring -,$(RELEASE_TAG)))
    PRE_RELEASE=true
endif
# the previous release tag, e.g., v0.3.9, excluding pre-release tags
PREVIOUS_TAG ?= $(shell git tag -l | grep -E "^v[0-9]+\.[0-9]+\.[0-9]+$$" | sort -V | grep -B1 $(RELEASE_TAG) | head -n 1 2>/dev/null)
## set by Prow, ref name of the base branch, e.g., main
RELEASE_ALIAS_TAG := $(PULL_BASE_REF)
RELEASE_DIR := out
RELEASE_NOTES_DIR := _releasenotes
GIT_REPO_NAME ?= cluster-api-addon-provider-helm
GIT_ORG_NAME ?= kubernetes-sigs
USER_FORK ?= $(shell git config --get remote.origin.url | cut -d/ -f4) # only works on https://github.com/<username>/cluster-api-addon-provider-helm.git style URLs
ifdef USER_FORK
USER_FORK := $(shell git config --get remote.origin.url | cut -d: -f2 | cut -d/ -f1) # for git@github.com:<username>/cluster-api-addon-provider-helm.git style URLs
endif
IMAGE_REVIEWERS ?= $(shell ./hack/get-project-maintainers.sh)

.PHONY: $(RELEASE_DIR)
$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

.PHONY: $(RELEASE_NOTES_DIR)
$(RELEASE_NOTES_DIR):
	mkdir -p $(RELEASE_NOTES_DIR)/

.PHONY: release
release: clean-release ## Build and push container images using the latest git tag for the commit
	@if [ -z "${RELEASE_TAG}" ]; then echo "RELEASE_TAG is not set"; exit 1; fi
	@if ! [ -z "$$(git status --porcelain)" ]; then echo "Your local git repository contains uncommitted changes, use git clean before proceeding."; exit 1; fi
	git checkout "${RELEASE_TAG}"
	# Set the manifest images to the staging/production bucket and Builds the manifests to publish with a release.
	$(MAKE) release-manifests-all
	$(MAKE) release-metadata

.PHONY: release-manifests-all
release-manifests-all: # Set the manifest images to the staging/production bucket and Builds the manifests to publish with a release.
	# Set the manifest image to the production bucket.
	$(MAKE) manifest-modification REGISTRY=$(PROD_REGISTRY)
	## Build the manifests
	$(MAKE) release-manifests
	## Clean the git artifacts modified in the release process
	$(MAKE) clean-release-git

.PHONY: manifest-modification
manifest-modification: # Set the manifest images to the staging/production bucket.
	$(MAKE) set-manifest-image \
		MANIFEST_IMG=$(REGISTRY)/$(IMAGE_NAME) MANIFEST_TAG=$(RELEASE_TAG) \
		TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy PULL_POLICY=IfNotPresent TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"


.PHONY: release-manifests
release-manifests: $(RELEASE_DIR) $(KUSTOMIZE) ## Build the manifests to publish with a release
	$(KUSTOMIZE) build config/default > $(RELEASE_DIR)/addon-components.yaml

.PHONY: release-metadata
release-metadata: $(RELEASE_DIR)
	cp metadata.yaml $(RELEASE_DIR)/metadata.yaml

.PHONY: release-binary
release-binary: $(RELEASE_DIR)
	docker run \
		--rm \
		-e CGO_ENABLED=0 \
		-e GOOS=$(GOOS) \
		-e GOARCH=$(GOARCH) \
		-v "$$(pwd):/workspace$(DOCKER_VOL_OPTS)" \
		-w /workspace \
		golang:$(GO_VERSION) \
		go build -a -trimpath -ldflags "$(LDFLAGS) -extldflags '-static'" \
		-o $(RELEASE_DIR)/$(notdir $(RELEASE_BINARY)) $(BUILD_PATH)

.PHONY: release-staging
release-staging: ## Build and push container images to the staging bucket
	REGISTRY=$(STAGING_REGISTRY) $(MAKE) docker-build-all docker-push-all release-alias-tag

.PHONY: release-staging-nightly
release-staging-nightly: ## Tag and push container images to the staging bucket. Example image tag: cluster-api-helm-controller:nightly_main_20210121
	$(eval NEW_RELEASE_ALIAS_TAG := nightly_$(RELEASE_ALIAS_TAG)_$(shell date +'%Y%m%d'))
	echo $(NEW_RELEASE_ALIAS_TAG)
	$(MAKE) release-alias-tag TAG=$(RELEASE_ALIAS_TAG) RELEASE_ALIAS_TAG=$(NEW_RELEASE_ALIAS_TAG)
	# Set the manifest image to the staging bucket.
	$(MAKE) manifest-modification REGISTRY=$(STAGING_REGISTRY) RELEASE_TAG=$(NEW_RELEASE_ALIAS_TAG)
	## Build the manifests
	$(MAKE) release-manifests
	# Example manifest location: artifacts.k8s-staging-cluster-api-helm.appspot.com/components/nightly_main_20210121/bootstrap-components.yaml
	gsutil cp $(RELEASE_DIR)/* gs://$(STAGING_BUCKET)/components/$(NEW_RELEASE_ALIAS_TAG)

.PHONY: release-alias-tag
release-alias-tag: ## Add the release alias tag to the last build tag
	gcloud container images add-tag $(CONTROLLER_IMG):$(TAG) $(CONTROLLER_IMG):$(RELEASE_ALIAS_TAG)

.PHONY: release-notes
release-notes: $(RELEASE_NOTES_DIR) $(RELEASE_NOTES)
	@if [ -n "${PRE_RELEASE}" ]; then \
	echo ":rotating_light: This is a RELEASE CANDIDATE. Use it only for testing purposes. If you find any bugs, file an [issue](https://github.com/kubernetes-sigs/cluster-api-addon-provider-helm/issues/new)." > $(RELEASE_NOTES_DIR)/release-notes-$(RELEASE_TAG).md; \
	else \
	$(RELEASE_NOTES) --org $(GIT_ORG_NAME) --repo $(GIT_REPO_NAME) --branch $(RELEASE_BRANCH)  --start-rev $(PREVIOUS_TAG) --end-rev $(RELEASE_TAG) --output $(RELEASE_NOTES_DIR)/tmp-release-notes.md --list-v2; \
	sed 's/\[SIG Cluster Lifecycle\]//g' $(RELEASE_NOTES_DIR)/tmp-release-notes.md > $(RELEASE_NOTES_DIR)/release-notes-$(RELEASE_TAG).md; \
	rm -f $(RELEASE_NOTES_DIR)/tmp-release-notes.md; \
	fi

.PHONY: promote-images
promote-images: $(KPROMO)
	$(KPROMO) pr --project cluster-api-helm --tag $(RELEASE_TAG) --reviewers "$(IMAGE_REVIEWERS)" --fork $(USER_FORK) --image cluster-api-helm-controller

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-push-all
docker-push-all: $(addprefix docker-push-,$(ALL_ARCH))  ## Push the docker images to be included in the release for all architectures + related multiarch manifests
	$(MAKE) docker-push-manifest

docker-push-%:
	$(MAKE) ARCH=$* docker-push

.PHONY: docker-push
docker-push: ## Push the docker images to be included in the release
	docker push $(CONTROLLER_IMG)-$(ARCH):$(TAG)

.PHONY: docker-push-manifest
docker-push-manifest: ## Push the multiarch manifest for the core docker images
	## Minimum docker version 18.06.0 is required for creating and pushing manifest images.
	docker manifest create --amend $(CONTROLLER_IMG):$(TAG) $(shell echo $(ALL_ARCH) | sed -e "s~[^ ]*~$(CONTROLLER_IMG)\-&:$(TAG)~g")
	@for arch in $(ALL_ARCH); do docker manifest annotate --arch $${arch} ${CONTROLLER_IMG}:${TAG} ${CONTROLLER_IMG}-$${arch}:${TAG}; done
	docker manifest push --purge $(CONTROLLER_IMG):$(TAG)
	$(MAKE) set-manifest-image MANIFEST_IMG=$(CONTROLLER_IMG) MANIFEST_TAG=$(TAG) TARGET_RESOURCE="./config/default/manager_image_patch.yaml"
	$(MAKE) set-manifest-pull-policy TARGET_RESOURCE="./config/default/manager_pull_policy.yaml"

.PHONY: set-manifest-pull-policy
set-manifest-pull-policy:
	$(info Updating kustomize pull policy file for manager resources)
	sed -i'' -e 's@imagePullPolicy: .*@imagePullPolicy: '"$(PULL_POLICY)"'@' $(TARGET_RESOURCE)

.PHONY: set-manifest-image
set-manifest-image:
	$(info Updating kustomize image patch file for manager resource)
	sed -i'' -e 's@image: .*@image: '"${MANIFEST_IMG}:$(MANIFEST_TAG)"'@' $(TARGET_RESOURCE)

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

##@ clean:

.PHONY: clean
clean: ## Remove generated binaries, GitBook files, Helm charts, and Tilt build files
	$(MAKE) clean-bin

.PHONY: clean-kind
clean-kind: ## Cleans up the kind cluster with the name $CAPI_KIND_CLUSTER_NAME
	kind delete cluster --name="$(CAPI_KIND_CLUSTER_NAME)" || true

.PHONY: clean-bin
clean-bin: ## Remove all generated binaries
	rm -rf $(BIN_DIR)
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHONY: clean-manifests ## Reset manifests in config directories back to main
clean-manifests:
	@read -p "WARNING: This will reset all config directories to local main. Press [ENTER] to continue."
	git checkout main config

.PHONY: clean-release-git
clean-release-git: ## Restores the git files usually modified during a release
	git restore ./*manager_image_patch.yaml ./*manager_pull_policy.yaml

.PHONY: clean-generated-yaml
clean-generated-yaml: ## Remove files generated by conversion-gen from the mentioned dirs. Example SRC_DIRS="./api/v1alpha4"
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name '*.yaml' -exec rm -f {} \;; done)

.PHONY: clean-generated-deepcopy
clean-generated-deepcopy: ## Remove files generated by conversion-gen from the mentioned dirs. Example SRC_DIRS="./api/v1alpha4"
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name 'zz_generated.deepcopy*' -exec rm -f {} \;; done)

.PHONY: clean-generated-conversions
clean-generated-conversions: ## Remove files generated by conversion-gen from the mentioned dirs. Example SRC_DIRS="./api/v1alpha4"
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name 'zz_generated.conversion*' -exec rm -f {} \;; done)

.PHONY: clean-generated-openapi-definitions
clean-generated-openapi-definitions: ## Remove files generated by openapi-gen from the mentioned dirs. Example SRC_DIRS="./api/v1alpha4"
	(IFS=','; for i in $(SRC_DIRS); do find $$i -type f -name 'zz_generated.openapi*' -exec rm -f {} \;; done)

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

.PHONY: $(CONTROLLER_GEN_BIN)
$(CONTROLLER_GEN_BIN): $(CONTROLLER_GEN) ## Build a local copy of controller-gen.

.PHONY: $(CONVERSION_GEN_BIN)
$(CONVERSION_GEN_BIN): $(CONVERSION_GEN) ## Build a local copy of conversion-gen.

.PHONY: $(OPENAPI_GEN_BIN)
$(OPENAPI_GEN_BIN): $(OPENAPI_GEN) ## Build a local copy of openapi-gen.

.PHONY: $(RUNTIME_OPENAPI_GEN_BIN)
$(RUNTIME_OPENAPI_GEN_BIN): $(RUNTIME_OPENAPI_GEN) ## Build a local copy of runtime-openapi-gen.

.PHONY: $(CONVERSION_VERIFIER_BIN)
$(CONVERSION_VERIFIER_BIN): $(CONVERSION_VERIFIER) ## Build a local copy of conversion-verifier.

.PHONY: $(GOTESTSUM_BIN)
$(GOTESTSUM_BIN): $(GOTESTSUM) ## Build a local copy of gotestsum.

.PHONY: $(GO_APIDIFF_BIN)
$(GO_APIDIFF_BIN): $(GO_APIDIFF) ## Build a local copy of go-apidiff

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST) ## Build a local copy of envsubst.

.PHONY: $(KUSTOMIZE_BIN)
$(KUSTOMIZE_BIN): $(KUSTOMIZE) ## Build a local copy of kustomize.

.PHONY: $(MOCKGEN_BIN)
$(MOCKGEN_BIN): $(MOCKGEN) ## Build a local copy of mockgen.

.PHONY: $(SETUP_ENVTEST_BIN)
$(SETUP_ENVTEST_BIN): $(SETUP_ENVTEST) ## Build a local copy of setup-envtest.

.PHONY: $(KPROMO_BIN)
$(KPROMO_BIN): $(KPROMO) ## Build a local copy of kpromo

.PHONY: $(RELEASE_NOTES_BIN)
$(RELEASE_NOTES_BIN): $(RELEASE_NOTES) ## Build a local copy of release notes

.PHONY: $(YQ_BIN)
$(YQ_BIN): $(YQ) ## Build a local copy of yq

.PHONY: $(TILT_PREPARE_BIN)
$(TILT_PREPARE_BIN): $(TILT_PREPARE) ## Build a local copy of tilt-prepare.

.PHONY: $(GOLANGCI_LINT_BIN)
$(GOLANGCI_LINT_BIN): $(GOLANGCI_LINT) ## Build a local copy of golangci-lint

.PHONY: $(GINKGO_BIN)
$(GINKGO_BIN): $(GINKGO) ## Build a local copy of ginkgo

$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(CONTROLLER_GEN_PKG) $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

## We are forcing a rebuilt of conversion-gen via PHONY so that we're always using an up-to-date version.
## We can't use a versioned name for the binary, because that would be reflected in generated files.
.PHONY: $(CONVERSION_GEN)
$(CONVERSION_GEN): # Build conversion-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(CONVERSION_GEN_PKG) $(CONVERSION_GEN_BIN) $(CONVERSION_GEN_VER)

$(CONVERSION_VERIFIER): $(TOOLS_DIR)/go.mod # Build conversion-verifier from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/$(CONVERSION_VERIFIER_BIN) sigs.k8s.io/cluster-api-addon-provider-helm/hack/tools/conversion-verifier

.PHONY: $(OPENAPI_GEN)
$(OPENAPI_GEN): # Build openapi-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(OPENAPI_GEN_PKG) $(OPENAPI_GEN_BIN) $(OPENAPI_GEN_VER)

## We are forcing a rebuilt of runtime-openapi-gen via PHONY so that we're always using an up-to-date version.
.PHONY: $(RUNTIME_OPENAPI_GEN)
$(RUNTIME_OPENAPI_GEN): $(TOOLS_DIR)/go.mod # Build openapi-gen from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/$(RUNTIME_OPENAPI_GEN_BIN) sigs.k8s.io/cluster-api-addon-provider-helm/hack/tools/runtime-openapi-gen

$(GOTESTSUM): # Build gotestsum from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GOTESTSUM_PKG) $(GOTESTSUM_BIN) $(GOTESTSUM_VER)

$(GO_APIDIFF): # Build go-apidiff from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GO_APIDIFF_PKG) $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

$(ENVSUBST): # Build gotestsum from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(ENVSUBST_PKG) $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(KUSTOMIZE): # Build kustomize from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(KUSTOMIZE_PKG) $(KUSTOMIZE_BIN) $(KUSTOMIZE_VER)

$(MOCKGEN): ## Build mockgen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) go.uber.org/mock/mockgen $(MOCKGEN_BIN) $(MOCKGEN_VER)

$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(SETUP_ENVTEST_PKG) $(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST_VER)

$(TILT_PREPARE): $(TOOLS_DIR)/go.mod # Build tilt-prepare from tools folder.
	cd $(TOOLS_DIR); go build -tags=tools -o $(BIN_DIR)/tilt-prepare sigs.k8s.io/cluster-api-addon-provider-helm/hack/tools/tilt-prepare

$(KPROMO):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(KPROMO_PKG) $(KPROMO_BIN) ${KPROMO_VER}

$(RELEASE_NOTES): ## Build release notes from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) k8s.io/release/cmd/release-notes $(RELEASE_NOTES_BIN) $(RELEASE_NOTES_VER)

$(YQ):
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(YQ_PKG) $(YQ_BIN) ${YQ_VER}

$(GOLANGCI_LINT): .github/workflows/golangci-lint.yml # Download golangci-lint using hack script into tools folder.
	hack/ensure-golangci-lint.sh \
		-b $(TOOLS_BIN_DIR) \
		$(shell yq e '.jobs.golangci.steps[] | select(.name? == "golangci-lint") | .with.version' .github/workflows/golangci-lint.yml )

$(GINKGO): # Build ginkgo from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GINKGO_PKG) $(GINKGO_BIN) $(GINGKO_VER)
