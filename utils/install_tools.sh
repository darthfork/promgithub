#!/usr/bin/env bash
GOSEC_VERSION="v2.22.7"
GOLANGCILINT_VERSION="v2.3.1"

# Install golangci-lint
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh |\
  sh -s -- -b $(go env GOPATH)/bin $GOLANGCILINT_VERSION

# Install gosec for static security analysis
curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh |\
  sh -s -- -b $(go env GOPATH)/bin $GOSEC_VERSION
