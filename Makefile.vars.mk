IMG_TAG ?= latest

VDCD_BRIDGE_MAIN_GO ?= .
VDCD_BRIDGE_GOOS ?= linux
VDCD_BRIDGE_GOARCH ?= amd64
VDCD_BRIDGE_GOARCH_ARM64 ?= arm64


CURDIR ?= $(shell pwd)
BIN_FILENAME ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/vdcd-bridge-amd64
BIN_FILENAME_ARM64 ?= $(CURDIR)/$(PROJECT_ROOT_DIR)/vdcd-bridge-arm64
WORK_DIR = $(CURDIR)/.work

go_bin ?= $(PWD)/.work/bin
$(go_bin):
	@mkdir -p $@

golangci_bin = $(go_bin)/golangci-lint


# Image URL to use all building/pushing image targets
VDCD_BRIDGE_GHCR_IMG ?= ghcr.io/splattner/vdcd-bridge:$(IMG_TAG)

