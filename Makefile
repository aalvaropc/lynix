APP=lynix

GOLANGCI_LINT_VERSION ?= v1.64.2
GOLANGCI_LINT = go run github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

.PHONY: dev tidy test lint build fmt

dev:
	go run ./cmd/lynix

tidy:
	go mod tidy

test:
	go test ./...

fmt:
	gofmt -w .

lint:
	$(GOLANGCI_LINT) run

build:
	go build -o bin/$(APP) ./cmd/lynix
