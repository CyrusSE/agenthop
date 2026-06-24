.PHONY: build test lint install clean

BINARY := agenthop
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -buildvcs=false -ldflags "-X github.com/CyrusSE/agenthop/internal/cli.version=$(VERSION)" -o bin/$(BINARY) ./cmd/agenthop

test:
	go test ./...

install:
	go install -buildvcs=false -ldflags "-X github.com/CyrusSE/agenthop/internal/cli.version=$(VERSION)" ./cmd/agenthop
	@rm -f $(HOME)/.local/bin/agenthop

clean:
	rm -rf bin/ dist/

lint:
	@which golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed, skipping"

release:
	goreleaser release --clean
