APP=lynix

.PHONY: dev tidy test lint build

dev:
	go run ./cmd/lynix

tidy:
	go mod tidy

test:
	go test ./...

lint:
	golangci-lint run

build:
	go build -o bin/$(APP) ./cmd/lynix
