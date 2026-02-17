APP=lynix

VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ' 2>/dev/null || echo unknown)

LDFLAGS = -X github.com/aalvaropc/lynix/internal/buildinfo.Version=$(VERSION) -X github.com/aalvaropc/lynix/internal/buildinfo.Commit=$(COMMIT) -X github.com/aalvaropc/lynix/internal/buildinfo.Date=$(DATE)

GOLANGCI_LINT_VERSION ?= v1.64.2
GOLANGCI_LINT = go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: dev tidy test lint build fmt

dev:
	go run -ldflags "$(LDFLAGS)" ./cmd/lynix

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	$(GOLANGCI_LINT) run

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/lynix
