.PHONY: build test clean fmt lint vet coverage examples

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Directories
SRC_DIR=./
EXAMPLES_DIR=./examples

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_FLAGS=-v

# Build the project
build:
	$(GOBUILD) $(BUILD_FLAGS) ./...

# Run all tests
test:
	go test -v ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GOCLEAN)
	@for dir in $(EXAMPLES_DIR)/*/; do \
		if [ -d "$${dir}" ]; then \
			name=$$(basename "$${dir%/}"); \
			rm -f "$${dir}$$name"; \
			rm -f "$${dir}$${name}_example"; \
		fi \
	done

# Build examples
examples:
	@echo "Building examples..."
	@for dir in $(EXAMPLES_DIR)/*/; do \
		if [ -f "$${dir}main.go" ]; then \
			name=$$(basename "$${dir%/}"); \
			echo "Building $$name..."; \
			$(GOBUILD) $(BUILD_FLAGS) -o "$${dir}$$name" "$${dir}"; \
		fi \
	done


# Install dependencies
deps:
	go mod download
	go mod tidy

# Run linter (requires golangci-lint)
lint:
	golangci-lint run ./...

# Run benchmarks
bench:
	go test -bench=. -benchmem ./...

# Generate documentation
docs:
	go doc -all > DOCUMENTATION.txt

# Check for security vulnerabilities (requires govulncheck)
security:
	govulncheck ./...

# All checks
check: fmt vet test

# Build and test everything
all: deps build test
