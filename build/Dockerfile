#
# OCI Secrets Store CSI Driver Provider
# 
# Copyright (c) 2022 Oracle America, Inc. and its affiliates.
# Licensed under the Universal Permissive License v 1.0 as shown at https://oss.oracle.com/licenses/upl/
#  

# FROM docker-remote.artifactory.oci.oraclecorp.com/oraclelinux:7-slim AS build
FROM golang:1.17.5 as build

# Odo Base Image Support used to configure yum:
# * pull rpms from artifactory by default and gpgcheck the repos that support it;
# * assume yes to all commands (no need to use '-y');
# * filter out all docs/man pages and languages except English;
# * exit non-zero to fail the image build if an rpm is missing from the repo.
# Documentation: https://confluence.oci.oraclecorp.com/display/DCPA/Odo+Base+Image+Support
# COPY --from=odo-docker-signed-local.artifactory.oci.oraclecorp.com/odo/base-image-support:ol7x-1.6 / /

# # Install build stage packages
# # RUN yum install net-tools
# RUN yum-config-manager --add-repo=https://artifactory.oci.oraclecorp.com/io-ol7-developer-golang117-yum-local && \
#     yum install golang-1.17.5-1.el7 make-3.82-24.el7 && \
#     yum clean all && \
#     rm -rf /var/cache/yum

WORKDIR /provider-src

# Copy provider sources
COPY go.mod go.sum Makefile  ./
COPY cmd       ./cmd
COPY internal  ./internal
COPY vendor    ./vendor

# Build provider
RUN make test build

# FROM docker-remote.artifactory.oci.oraclecorp.com/oraclelinux:7-slim
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