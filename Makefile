.PHONY: help test test-coverage test-race lint fmt vet clean examples

# Default target
help:
	@echo "Available targets:"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage report"
	@echo "  test-race     - Run tests with race detector"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format code with gofmt"
	@echo "  vet           - Run go vet"
	@echo "  clean         - Remove build artifacts"
	@echo "  examples      - Build all examples"

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -coverprofile=coverage.txt -covermode=atomic ./...
	go tool cover -html=coverage.txt -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run tests with race detector
test-race:
	go test -v -race ./...

# Run linter
lint:
	golangci-lint run

# Format code
fmt:
	gofmt -s -w .
	goimports -w .

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	rm -f coverage.txt coverage.html
	rm -rf dist/ build/ bin/
	find . -name "*.test" -delete
	find . -name "*.out" -delete

# Build examples
examples:
	@echo "Building examples..."
	go build -o bin/basic examples/basic/main.go
	go build -o bin/template examples/template/main.go
	go build -o bin/attachment examples/attachment/main.go
	go build -o bin/batch examples/batch/main.go
	@echo "Examples built in bin/"

# Install development tools
install-tools:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install golang.org/x/tools/cmd/goimports@latest

# Run all checks (for CI)
ci: fmt vet lint test-race
	@echo "All CI checks passed!"
