.PHONY: build container cross-platform debug release test go-version coverage fmt lint mod clean

include version

.DEFAULT_GOAL		:= help
TARGET			:= promgithub
SRC			:= ./...
LDFLAGS			:= -X main.Version=$(VERSION) -s -w
LDFLAGS_DBG		:= -X main.Version=$(VERSION)
BUILDDIR		:= build
REGISTRY		:= ghcr.io/darthfork/promgithub

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile\
		| awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

mkdir:
	@mkdir -p $(BUILDDIR)

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

cross-platform: ## Create cross-platform binaries
cross-platform: mkdir
	for GOARCH in amd64 arm64; do \
		GOOS=linux GOARCH=$$GOARCH $(MAKE) TARGET=$(TARGET)-linux-$$GOARCH-$(VERSION) build; \
	done

test: ## Run unit tests
test: PROMGITHUB_WEBHOOK_SECRET := test-secret
test:
	@go test -v $(SRC)

lint: ## Run linter
	@golangci-lint run -v\
		--config=./.golangci.yaml\
		--timeout=5m\
		--out-format=colored-line-number

fmt: ## Format golang source files
	@go fmt ./...

coverage: ## Run unit tests with coverage
	@go test -cover -v $(SRC) -coverprofile=coverage.out
	@go tool cover -html=coverage.out

build-container:
	@docker build --progress=plain -t $(REGISTRY):$(VERSION) .

push-container: build-container
	@docker push $(REGISTRY):$(VERSION)

container: ## Build promgithub service container
container: build-container

release: ## Create github release and upload artifacts
release: push-container cross-platform
	@gh release create v$(VERSION)\
		--title "promgithub-v$(VERSION)"\
		--generate-notes\
		$(BUILDDIR)/*

clean: ## Clean build directory
	@rm -rf $(BUILDDIR)
