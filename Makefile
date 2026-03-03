.PHONY: fmt lint build oapi proto

# Default target
all: test

# Format code
fmt:
	golangci-lint fmt

# Lint code (requires golangci-lint to be installed)
lint:
	golangci-lint run --fix

# Generate OpenAPI server code from spec
oapi:
	go generate tools.go

build:
	go build

proto:
	protoc --go_out=. --go-grpc_out=. proto/*.proto
