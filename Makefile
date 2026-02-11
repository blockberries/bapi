.PHONY: build test test-race schema lint clean

# Build all packages.
build:
	go build ./...

# Run all tests.
test:
	go test ./... -count=1

# Run tests with race detector.
test-race:
	go test ./... -race -count=1

# Validate cramberry schemas (if cramberry CLI is available).
schema:
	@echo "Cramberry schemas live in grpc/schema/bapi/v1/"
	@echo "  types.cram   — wire format field assignments"
	@echo "  service.cram — service definition"
	@if command -v cramberry >/dev/null 2>&1; then \
		cramberry check grpc/schema/bapi/v1/*.cram; \
	else \
		echo "cramberry CLI not found; skipping schema validation"; \
	fi

# Run linter.
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		go vet ./...; \
	fi

# Clean build artifacts.
clean:
	go clean ./...
