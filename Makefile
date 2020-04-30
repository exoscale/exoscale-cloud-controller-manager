include go.mk/init.mk

VERSION := $(shell git describe --tags --always 2> /dev/null || cat $(CURDIR)/.version 2> /dev/null || echo 0)
export VERSION

GO_MAIN_PKG_PATH := "./cmd/exoscale-cloud-controller-manager"

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: docker
docker:
	docker build --rm \
		-t exoscale/cloud-controller-manager \
		--build-arg VERSION="${VERSION}" \
		--build-arg VCS_REF="${GIT_REVISION}" \
		--build-arg BUILD_DATE="$(shell date -u +"%Y-%m-%dT%H:%m:%SZ")" \
		.
	docker tag exoscale/cloud-controller-manager:latest exoscale/cloud-controller-manager:${VERSION}

docker-push:
	docker push exoscale/cloud-controller-manager:latest && docker push exoscale/cloud-controller-manager:${VERSION}
