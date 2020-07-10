include go.mk/init.mk
include go.mk/public.mk

PROJECT_URL = https://github.com/exoscale/exoscale-cloud-controller-manager

GO_MAIN_PKG_PATH := "./cmd/exoscale-cloud-controller-manager"

.PHONY: docker
docker:
	docker build --rm \
		-t exoscale/cloud-controller-manager \
		--build-arg VERSION="${VERSION}" \
		--build-arg VCS_REF="${GIT_REVISION}" \
		--build-arg BUILD_DATE="$(shell date -u +"%Y-%m-%dT%H:%m:%SZ")" \
		.
	docker tag exoscale/cloud-controller-manager:latest exoscale/cloud-controller-manager:${VERSION}

.PHONY: docker-push
docker-push:
	docker push exoscale/cloud-controller-manager:latest && docker push exoscale/cloud-controller-manager:${VERSION}

.PHONY: integtest
integtest:
	@INCLUDE_PATH=$(PWD) ./integtest/run.bash
