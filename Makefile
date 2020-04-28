include go.mk/init.mk

VERSION := $(shell git describe --tags --always 2> /dev/null || cat $(CURDIR)/.version 2> /dev/null || echo 0)
export VERSION

GO_MAIN_PKG_PATH := "./cmd/exoscale-cloud-controller-manager"

.PHONY: version
version:
	@echo $(VERSION)
