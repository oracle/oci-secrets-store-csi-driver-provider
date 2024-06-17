#
# OCI Secrets Store CSI Driver Provider
# 
# Copyright (c) 2022 Oracle America, Inc. and its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
#
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

all: lint test build

lint:
	golangci-lint run

vet:
	go vet ./...

staticcheck:
	# install if doesn't exist `go install honnef.co/go/tools/cmd/staticcheck@latest`
	staticcheck ./...

# static code analysis
sca: lint vet staticcheck

test:
	go test ./...

build: cmd/server/main.go
	go build -ldflags $(LDFLAGS) -mod vendor -o dist/provider ./cmd/server/main.go

docker-build-push:
	docker buildx build --push --platform=linux/amd64,linux/arm64 -t ${IMAGE_PATH} -f build/Dockerfile .   

print-docker-image-path:
	@echo ${IMAGE_PATH}

test-coverage:
	go test -coverprofile=cover.out ./…
	go tool cover -html=cover.out