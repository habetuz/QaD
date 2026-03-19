.PHONY: fmt lint build oapi proto push-ghcr

# Allows: make push-ghcr latest 1.0.0
PUSH_GHCR_TAGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

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

push-ghcr:
	@if [ -z "$(PUSH_GHCR_TAGS)" ]; then \
		echo "Usage: make push-ghcr <tag1> [tag2 ...] [IMAGE_NAME=ghcr.io/<owner>/qad]"; \
		exit 1; \
	fi
	IMAGE_NAME="$(IMAGE_NAME)" ./push-ghcr.sh $(PUSH_GHCR_TAGS)

# Swallow extra make goals so positional tags aren't treated as targets.
%:
	@:
