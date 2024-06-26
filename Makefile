# Set Shell to bash, otherwise some targets fail with dash/zsh etc.
SHELL := /bin/bash

# Disable built-in rules
MAKEFLAGS += --no-builtin-rules
MAKEFLAGS += --no-builtin-variables
.SUFFIXES:
.SECONDARY:
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help
	@grep -E -h '\s##\s' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# extensible array of targets. Modules can add target to this variable for the all-in-one target.
clean_targets := build-clean

PROJECT_ROOT_DIR = .
include Makefile.vars.mk

go_build ?= go build -o $(BIN_FILENAME) $(VDCD_BRIDGE_MAIN_GO)

go_build_arm64 ?= go build -o $(BIN_FILENAME_ARM64) $(VDCD_BRIDGE_MAIN_GO)

.PHONY: test
test: ## Run tests
	go test ./... -coverprofile cover.out

.PHONY: build
build: fmt vet $(BIN_FILENAME)


.PHONY: fmt
fmt: ## Run go fmt against code
	go fmt ./

.PHONY: vet
vet: ## Run go vet against code
	go vet ./

.PHONY: lint
lint: fmt vet golangci-lint ## Invokes all linting targets
	@echo 'Check for uncommitted changes ...'
	git diff --exit-code

.PHONY: golangci-lint
golangci-lint: $(golangci_bin) ## Run golangci linters
	$(golangci_bin) run --timeout 5m --out-format colored-line-number ./...

.PHONY: docker-build
docker-build: docker-build-amd64 docker-build-arm64

.PHONY: docker-build-amd64
docker-build-amd64: $(BIN_FILENAME) ## Build the docker image linux/amd64
	docker build . \
	    -f build/Dockerfile \
		--tag $(VDCD_BRIDGE_GHCR_IMG)-amd64 \
		--platform linux/amd64 \
		--build-arg VDCD_BRIDGE_BIN=vdcd-bridge-amd64
		
.PHONY: docker-build-arm64
docker-build-arm64: $(BIN_FILENAME_ARM64) ## Build the docker image linux/arm64
	docker build . \
	    -f build/Dockerfile \
		--tag $(VDCD_BRIDGE_GHCR_IMG)-arm64 \
		--platform linux/arm64 \
		--build-arg VDCD_BRIDGE_BIN=vdcd-bridge-arm64

.PHONY: docker-manifest
docker-manifest: docker-manifest-create docker-manifest-push # Create and push docker manifest

.PHONY:docker-manifest-create
docker-manifest-create: ## Create the docker manifest
	docker manifest create $(VDCD_BRIDGE_GHCR_IMG) \
	$(VDCD_BRIDGE_GHCR_IMG)-amd64 \
	$(VDCD_BRIDGE_GHCR_IMG)-arm64 

.PHONY: docker-manifest-push
docker-manifest-push: ## Push the docker manifest
	docker manifest push $(VDCD_BRIDGE_GHCR_IMG)


.PHONY: docker-push
docker-push: ## Push the docker image
	docker push $(VDCD_BRIDGE_GHCR_IMG)-amd64
	docker push $(VDCD_BRIDGE_GHCR_IMG)-arm64

build-clean:
	rm -rf dist/ bin/ cover.out $(BIN_FILENAME) $(BIN_FILENAME_ARM64) $(WORK_DIR)

clean: $(clean_targets) ## Cleans up all the locally generated resources


###
### Assets
###

# Build the binary without running generators
.PHONY: $(BIN_FILENAME)
$(BIN_FILENAME): export CGO_ENABLED = 0
$(BIN_FILENAME): export GOOS = $(VDCD_BRIDGE_GOOS)
$(BIN_FILENAME): export GOARCH = $(VDCD_BRIDGE_GOARCH)
$(BIN_FILENAME):
	$(go_build)

.PHONY: $(BIN_FILENAME_ARM64)
$(BIN_FILENAME_ARM64): export CGO_ENABLED = 0
$(BIN_FILENAME_ARM64): export GOOS = $(VDCD_BRIDGE_GOOS)
$(BIN_FILENAME_ARM64): export GOARCH = $(VDCD_BRIDGE_GOARCH_ARM64)
$(BIN_FILENAME_ARM64):
	$(go_build_arm64)

$(golangci_bin): | $(go_bin)
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go_bin)"