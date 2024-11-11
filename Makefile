.PHONY: build container cross-platform debug release test go-version coverage fmt lint mod clean

include version

.DEFAULT_GOAL		:= help
CI			?= false
USERNAME		:= darthfork
TARGET			:= promgithub
SRC			:= ./...
LDFLAGS			:= -X main.Version=$(VERSION) -s -w
LDFLAGS_DBG		:= -X main.Version=$(VERSION)
BUILDDIR		:= build
TARGETARCH		:= linux/amd64,linux/arm64
CHART_SOURCE		:= helm/promgithub
CONTAINER_REGISTRY	:= ghcr.io/$(USERNAME)/$(TARGET)
CHART_REGISTRY		:= oci://ghcr.io/$(USERNAME)/$(TARGET)-charts
CHART_VERSION		:= $(shell grep 'version:' $(CHART_SOURCE)/Chart.yaml | tail -n1 | awk '{ print $$2 }')

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile\
		| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'

mkdir:
	@mkdir -p $(BUILDDIR)

setup-commit-hooks: ## Setup git commit hooks
	@mkdir -p .git/hooks
	@cp .github/hooks/* .git/hooks/

mod: ## Update go modules
	@go mod tidy
	@go mod verify

go-version: ## Get the Go version from go.mod
	@grep '^go ' go.mod | awk '{print $$2}'

build: ## Build promgithub service binary
build: CGO_ENABLED := 0
build: mkdir
	@go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(TARGET) $(SRC)

debug: ## Build promgithub service binary with debug information
debug: LDFLAGS := $(LDFLAGS_DBG)
debug: TARGET := $(TARGET)-debug
debug: build

test: ## Run unit tests
test: PROMGITHUB_WEBHOOK_SECRET := test-secret
test: mod
	@go test -v $(SRC)

container: ## Build promgithub service container
	@docker build --progress=plain -t $(CONTAINER_REGISTRY):$(VERSION) .

lint: ## Run linter
	@golangci-lint run -v\
		--config=./.golangci.yaml\
		--timeout=5m\
		--out-format=colored-line-number

fmt: ## Format golang source files
	@go fmt $(SRC)

coverage: ## Run unit tests with coverage
	@go test -cover -v $(SRC) -coverprofile=coverage.out
	@go tool cover -html=coverage.out

ci-check:
	@if [ "$(CI)" = "false" ]; then \
		printf "\033[31mError: This target is only intended for CI builds\n\n"; \
		printf "\033[0mTo override this lock, run \033[32m\"CI=true make <your-target>\" \n\n\033[0m"; \
		exit 1 >/dev/null 2>&1; \
	fi

build-cross-platform-binaries: ci-check mkdir
	@for GOARCH in amd64 arm64; do \
		GOOS=linux GOARCH=$$GOARCH $(MAKE) TARGET=$(TARGET)-linux-$$GOARCH-$(VERSION) build; \
	done

build-cross-platform-container: ci-check
	@docker buildx build \
		--platform linux/amd64,linux/arm64 \
		-t $(CONTAINER_REGISTRY):$(VERSION) \
		--cache-from type=gha,scope=$(TARGET) \
		--cache-to type=gha,mode=max,scope=$(TARGET) \
		. --push

build-and-push-helm-chart: ci-check mkdir
	@helm package $(CHART_SOURCE) -d $(BUILDDIR)
	@helm push $(BUILDDIR)/$(TARGET)-$(CHART_VERSION).tgz $(CHART_REGISTRY)

create-github-release: ci-check
	@gh release create v$(VERSION) \
		--title "$(TARGET)-v$(VERSION)" \
		--generate-notes \
		$(BUILDDIR)/*


release: ## Create cross-platform binaries, containers, helm chart and release to Github (CI only)
release: ci-check
release: build-cross-platform-binaries
release: build-cross-platform-container
release: build-and-push-helm-chart
release: create-github-release

clean: ## Clean build directory
	@rm -rf $(BUILDDIR)
