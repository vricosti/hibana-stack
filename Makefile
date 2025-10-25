.PHONY: build install clean test deps

# Build the hibana binary
build:
	@echo "Building hibana..."
	go build -o bin/hibana ./cmd/hibana

# Install hibana to /usr/local/bin
install: build
	@echo "Installing hibana to /usr/local/bin..."
	sudo cp bin/hibana /usr/local/bin/
	@echo "✓ hibana installed successfully"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf bin/
	go clean

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Development build with race detection
dev:
	@echo "Building with race detection..."
	go build -race -o bin/hibana ./cmd/hibana

# Create release binary
release:
	@echo "Building release binary..."
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/hibana ./cmd/hibana
	@echo "✓ Release binary created at bin/hibana"
