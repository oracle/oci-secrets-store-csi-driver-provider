# OCI Secrets Store CSI Driver Provider
#
# Copyright (c) 2022 Oracle America, Inc. and its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
#

FROM golang:1.19 as build

WORKDIR /provider-src

# Copy provider sources
COPY go.mod go.sum Makefile  ./
COPY cmd       ./cmd
COPY internal  ./internal
COPY vendor    ./vendor

# Build provider
RUN make test build

FROM oraclelinux:7-slim

LABEL author="OKE Foundations Team"

COPY --from=build /provider-src/dist/provider /opt/provider/bin/provider

# copy license files
COPY LICENSE.txt /opt/provider/licenses/LICENSE.txt
COPY THIRD_PARTY_LICENSES.txt /opt/provider/licenses/THIRD_PARTY_LICENSES.txt

# Note that the provider container should run as root in order to
# mount the Kubernetes hostPath volume and create Unix Domain Socket in that volume.

WORKDIR /opt/provider

ENTRYPOINT ["/opt/provider/bin/provider"]
