#!/usr/bin/env bash

# Tool versions
GOSEC_VERSION="v2.22.7"
GOLANGCILINT_VERSION="v2.3.1"
TRIVY_VERSION="latest"
GOVULNCHECK_VERSION="latest"

echo "Installing security and development tools..."

# Install golangci-lint
echo "Installing golangci-lint $GOLANGCILINT_VERSION..."
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh |\
  sh -s -- -b $(go env GOPATH)/bin $GOLANGCILINT_VERSION

# Install gosec for static security analysis
echo "Installing gosec $GOSEC_VERSION..."
curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh |\
  sh -s -- -b $(go env GOPATH)/bin $GOSEC_VERSION

# Install Trivy for container security scanning
echo "Installing Trivy $TRIVY_VERSION..."
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh |\
  sh -s -- -b $(go env GOPATH)/bin $TRIVY_VERSION

# Install govulncheck for vulnerability scanning
echo "Installing govulncheck $GOVULNCHECK_VERSION..."
go install golang.org/x/vuln/cmd/govulncheck@$GOVULNCHECK_VERSION

echo "All tools installed successfully!"
