.PHONY: build container cross-platform debug release test test-all go-version coverage fmt lint deps security mod clean dev-setup

include version

.DEFAULT_GOAL		:= help
CI			?= false
COLOR_CYAN		:= \033[36m
COLOR_RESET		:= \033[0m
COLOR_GREEN		:= \033[32m
COLOR_RED		:= \033[31m
USERNAME		:= darthfork
TARGET			:= promgithub
SRC			:= ./...
LDFLAGS			:= -X main.Version=$(VERSION) -s -w
LDFLAGS_DBG		:= -X main.enableDebug=true -X main.Version=$(VERSION)
BUILDDIR		:= build
ARCHS			:= amd64 arm64
PLATFORMS		:= linux/amd64,linux/arm64
CHART_SOURCE		:= helm/promgithub
CONTAINER_REGISTRY	:= ghcr.io/$(USERNAME)/$(TARGET)
CHART_REGISTRY		:= oci://ghcr.io/$(USERNAME)/$(TARGET)-charts
CHART_VERSION		:= $(shell grep 'version:' $(CHART_SOURCE)/Chart.yaml | tail -n1 | awk '{ print $$2 }')

build: ## Build promgithub service binary
build: CGO_ENABLED := 0
build: deps mkdir
	@go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(TARGET) $(SRC)

debug: ## Build promgithub service binary with debug information
debug: LDFLAGS := $(LDFLAGS_DBG)
debug: TARGET := $(TARGET)-debug
debug: build

test: ## Run unit tests
test: PROMGITHUB_WEBHOOK_SECRET := test-secret
test:
	@go test -v $(SRC)

coverage: ## Run unit tests with coverage
	@go test -race -coverprofile=coverage.out -covermode=atomic $(SRC)
	@go tool cover -html=coverage.out -o coverage.html


lint: ## Lint golang source files
	@golangci-lint run -v \
		--config=./.golangci.yaml \
		--timeout=5m \
		--out-format=colored-line-number

fmt: ## Format golang source files
	@go fmt $(SRC)

mod: ## Update go modules
	@go mod tidy
	@go mod verify

deps: mod ## Install/update dependencies
	@go mod download

container: ## Build promgithub service container
	@docker build --progress=plain -t $(CONTAINER_REGISTRY):$(VERSION) .

package-helm-chart: mkdir ## Package promgithub helm chart
	@helm package $(CHART_SOURCE) -d $(BUILDDIR)

build-cross-platform-binaries: mkdir
	@set -e; for GOARCH in $(ARCHS); do \
		echo "${COLOR_GREEN}Building $(TARGET)-linux-$$GOARCH-$(VERSION)${COLOR_RESET}"; \
		GOOS=linux GOARCH=$$GOARCH TARGET=$(TARGET)-linux-$$GOARCH-$(VERSION) $(MAKE) build; \
	done

build-cross-platform-container: ci-check
	@docker buildx build \
		--platform $(PLATFORMS)\
		-t $(CONTAINER_REGISTRY):$(VERSION) \
		--cache-from type=gha,scope=$(TARGET) \
		--cache-to type=gha,mode=max,scope=$(TARGET) \
		. --push

release-helm-chart: ci-check package-helm-chart
	@helm push $(BUILDDIR)/$(TARGET)-$(CHART_VERSION).tgz $(CHART_REGISTRY)

create-github-release: ci-check
	@gh release create v$(VERSION) \
		--title "$(TARGET)-v$(VERSION)" \
		--generate-notes \
		$(BUILDDIR)/*

security: ## Run security checks
	@go run golang.org/x/vuln/cmd/govulncheck@latest ./...
	@gosec ./...

test-all: test coverage security lint ## Run all tests and checks

dev-setup: deps setup-commit-hooks ## Setup development environment
	@echo "Development environment ready"

release: ## Create cross-platform binaries, containers, helm chart and release to Github (CI only)
release: ci-check
release: build-cross-platform-binaries
release: build-cross-platform-container
release: release-helm-chart
release: create-github-release

setup-commit-hooks:
	@mkdir -p .git/hooks
	@cp .github/hooks/* .git/hooks/

go-version:
	@grep '^go ' go.mod | awk '{print $$2}'

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile\
		| awk 'BEGIN {FS = ":.*?## "}; {printf "${COLOR_CYAN}%-20s${COLOR_RESET} %s\n", $$1, $$2}'

mkdir:
	@mkdir -p $(BUILDDIR)

ci-check:
	@if [ "$(CI)" = "false" ]; then \
		printf "${COLOR_RED}Error: This target is only intended for CI builds\n\n${COLOR_RESET}"; \
		printf "${COLOR_RESET}To override this lock, run ${COLOR_GREEN}\"CI=true make <your-target>\" \n\n${COLOR_RESET}"; \
		exit 1 >/dev/null 2>&1; \
	fi

clean: ## Clean build directory
	@rm -rf $(BUILDDIR)
	@rm -f coverage.out coverage.html

