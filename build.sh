#!/bin/bash

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if Go is installed
check_go() {
    if ! command -v go &> /dev/null; then
        echo -e "${RED}Error: Go is not installed${NC}"
        echo ""
        echo "To install Go on Ubuntu/Debian, run:"
        echo "  sudo apt update"
        echo "  sudo apt install golang-go"
        echo ""
        echo "Or download the latest version from https://go.dev/dl/"
        exit 1
    fi

    GO_VERSION=$(go version | awk '{print $3}')
    echo -e "${GREEN}✓${NC} Found Go: $GO_VERSION"
}

# Build the hibana CLI binary
build_cli() {
    echo "Building hibana CLI..."
    go build -buildvcs=false -o bin/hibana ./cmd/hibana
    echo -e "${GREEN}✓${NC} CLI build successful: bin/hibana"
}

# Build API binary and frontend for deployment
build_api() {
    echo "Building API for deployment..."

    # Create output directory
    mkdir -p ansible/api-build/web

    # Build the API binary
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o ansible/api-build/hibana-api ./cmd/api

    # Check if frontend is built
    if [ ! -d "web/admin/dist" ]; then
        echo -e "${YELLOW}Warning: Frontend not built. Run 'cd web/admin && npm install && npm run build' first${NC}"
        echo -e "${YELLOW}Skipping frontend copy${NC}"
    else
        cp -r web/admin/dist/* ansible/api-build/web/
    fi

    # Create Dockerfile for pre-built binary
    cat > ansible/api-build/Dockerfile << 'EOF'
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

# Copy pre-built binary
COPY hibana-api ./hibana-api

# Copy frontend files
COPY web/ ./web/

EXPOSE 3000

CMD ["./hibana-api", "--port", "3000", "--static", "./web"]
EOF

    echo -e "${GREEN}✓${NC} API build successful: ansible/api-build/"
}

# Build everything (CLI + API)
build() {
    echo "Building hibana..."
    check_go
    build_cli
    build_api
    echo -e "${GREEN}✓${NC} Build complete"
}

# Install hibana to /usr/local/bin
install() {
    build
    echo "Installing hibana to /usr/local/bin..."
    sudo cp bin/hibana /usr/local/bin/
    echo -e "${GREEN}✓${NC} hibana installed successfully"
}

# Clean build artifacts
clean() {
    echo "Cleaning..."
    rm -rf bin/
    rm -rf ansible/api-build/
    if command -v go &> /dev/null; then
        go clean
    fi
    echo -e "${GREEN}✓${NC} Cleaned build artifacts"
}

# Download dependencies
deps() {
    echo "Downloading dependencies..."
    check_go
    go mod download
    go mod tidy
    echo -e "${GREEN}✓${NC} Dependencies updated"
}

# Run tests
test() {
    echo "Running tests..."
    check_go
    go test ./...
}

# Development build with race detection
dev() {
    echo "Building with race detection..."
    check_go
    go build -buildvcs=false -race -o bin/hibana ./cmd/hibana
    echo -e "${GREEN}✓${NC} Development build created at bin/hibana"
}

# Create release binary
release() {
    echo "Building release binary..."
    check_go
    CGO_ENABLED=0 go build -buildvcs=false -ldflags="-s -w" -o bin/hibana ./cmd/hibana
    echo -e "${GREEN}✓${NC} Release binary created at bin/hibana"
}

# Show usage
usage() {
    echo "Hibana Build Script"
    echo ""
    echo "Usage: $0 [command]"
    echo ""
    echo "Run without arguments to build the binary."
    echo ""
}

# Main script logic
# Default to build if no command specified
COMMAND="${1:-build}"

case "$COMMAND" in
    build)
        build
        ;;
    install)
        install
        ;;
    clean)
        clean
        ;;
    deps)
        deps
        ;;
    test)
        test
        ;;
    dev)
        dev
        ;;
    release)
        release
        ;;
    help|--help|-h)
        usage
        ;;
    *)
        echo -e "${RED}Unknown command: $1${NC}"
        echo ""
        usage
        exit 1
        ;;
esac
