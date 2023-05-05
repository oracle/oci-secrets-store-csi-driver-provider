#
# OCI Secrets Store CSI Driver Provider
# 
# Copyright (c) 2022 Oracle America, Inc. and its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
#

# HELP
# This will output the help for each task
# thanks to https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
.PHONY: help

help: ## This help.
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help


$(eval BUILD_DATE=$(shell date -u +%Y.%m.%d.%H.%M))
$(eval GIT_TAG=$(shell git log -n 1 --pretty=format:"%H"))
BUILD_VERSION=$(GIT_TAG)-$(BUILD_DATE)
IMAGE_REPO_NAME=oci-secrets-store-csi-driver-provider

ifeq "$(IMAGE_REGISTRY)" ""
	IMAGE_REGISTRY  ?= ghcr.io/oracle-samples
else
	IMAGE_REGISTRY	?= ${IMAGE_REGISTRY}
endif

# IMAGE_REPO=$(IMAGE_REGISTRY)/oci-secrets-store-csi-driver-provider
IMAGE_URL=$(IMAGE_REGISTRY)/$(IMAGE_REPO_NAME)
IMAGE_TAG=$(GIT_TAG)
IMAGE_PATH=$(IMAGE_URL):$(IMAGE_TAG)

LDFLAGS?="-X github.com/oracle-samples/oci-secrets-store-csi-driver-provider/internal/server.BuildVersion=$(BUILD_VERSION)"

.PHONY : lint test build

all: lint test build ## lint, test and build the code

lint: ## run golangci-lint
	golangci-lint run

vet: ## run go vet
	go vet ./...

staticcheck: ## run static check
	# install if doesn't exist `go install honnef.co/go/tools/cmd/staticcheck@latest`
	staticcheck ./...

# static code analysis
sca: lint vet staticcheck ## static code analysis

test: ## test run go test
	go test ./...

build: cmd/server/main.go ## build
	go build -ldflags $(LDFLAGS) -mod vendor -o dist/provider ./cmd/server/main.go

docker-build: ## build container image
	docker build -t ${IMAGE_PATH} -f build/Dockerfile .
	# docker buildx build --platform=linux/amd64 -t ${IMAGE_PATH} -f build/Dockerfile .   

docker-push: ## push container image
	docker push ${IMAGE_PATH}

docker-build-push: docker-build ## build and push container image
	docker push ${IMAGE_PATH}

print-docker-image-path: ## print container image path
	@echo ${IMAGE_PATH}

test-coverage: ## run test coverage
	go test -coverprofile=cover.out ./…
	go tool cover -html=cover.out