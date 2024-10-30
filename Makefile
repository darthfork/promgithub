.PHONY: build container

include version

.DEFAULT_GOAL	:= all
TARGET		:= promgithub
SRC		:= ./...
LDFLAGS		:= -X main.Version=$(VERSION) -s -w
LDFLAGS_DBG	:= -X main.Version=$(VERSION)
BUILDDIR	:= build

all: build

mkdir:
	@mkdir -p $(BUILDDIR)

build: CGO_ENABLED := 0
build: mkdir
	@go build -ldflags "$(LDFLAGS)" -o $(BUILDDIR)/$(TARGET) $(SRC)

debug: LDFLAGS := $(LDFLAGS_DBG)
debug: TARGET := $(TARGET)-debug
debug: all

test: GITHUB_WEBHOOK_SECRET := test-secret
test:
	@go test -v $(SRC)

fmt:
	@go fmt ./...

lint:
	@golangci-lint run

mod:
	@go mod tidy
	@go mod verify

clean:
	@rm -rf $(BUILDDIR)
