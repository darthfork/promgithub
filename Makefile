.PHONY: build container

include version

.DEFAULT_GOAL	:= help
TARGET		:= promgithub
SRC		:= ./...
LDFLAGS		:= -X main.Version=$(VERSION) -s -w
LDFLAGS_DBG	:= -X main.Version=$(VERSION)
BUILDDIR	:= build

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' Makefile | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-10s\033[0m %s\n", $$1, $$2}'

mkdir:
	@mkdir -p $(BUILDDIR)

build: ## Build promgithub service binary
build: CGO_ENABLED := 0
build: mkdir
	@go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(TARGET) $(SRC)

debug: ## Build promgithub service binary with debug information
debug: LDFLAGS := $(LDFLAGS_DBG)
debug: TARGET := $(TARGET)-debug
debug: all

test: GITHUB_WEBHOOK_SECRET := test-secret
test: ## Run unit tests
	@go test -v $(SRC)

coverage: ## Run unit tests with coverage
	@go test -cover -v $(SRC) -coverprofile=coverage.out
	@go tool cover -html=coverage.out

fmt: ## Format golang source files
	@go fmt ./...

lint: ## Run linter
	@golangci-lint run

mod: ## Update go modules
	@go mod tidy
	@go mod verify

clean: ## Clean build directory
	@rm -rf $(BUILDDIR)
