## Project

PACKAGE := github.com/exoscale/exoscale-cloud-controller-manager
PROJECT_URL := https://$(PACKAGE)
GO_MAIN_PKG_PATH := ./cmd/exoscale-cloud-controller-manager

EXTRA_ARGS := -parallel 3 -count=1 -failfast

# Dependencies

# Requires: https://github.com/exoscale/go.mk
# - install: git submodule update --init --recursive go.mk
# - update:  git submodule update --remote
include go.mk/init.mk
include go.mk/public.mk


## Targets

# Docker
include Makefile.docker
