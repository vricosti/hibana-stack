#!/bin/bash

set -e

# Check root
if [ "$EUID" -ne 0 ]; then
    echo "Please run as root: sudo ./setup.sh"
    exit 1
fi

# Check/install Go
if ! command -v go &> /dev/null; then
    if [ -f /etc/debian_version ]; then
        echo "Debian/Ubuntu detected. Go is not installed."
        read -p "Run 'apt update && apt install -y golang-go'? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            apt update && apt install -y golang-go
        else
            echo "Aborted."
            exit 1
        fi
    else
        echo "Please install Go manually: https://go.dev/dl/"
        exit 1
    fi
fi
echo "Go: $(go version)"

# Build CLI
echo "Building hibana CLI..."
go build -buildvcs=false -o hibana ./cmd/hibana

# Build API
echo "Building hibana API..."
mkdir -p ansible/api-build/web
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -buildvcs=false -ldflags="-s -w" -o ansible/api-build/hibana-api ./cmd/api

# Build frontend if Node available, otherwise use pre-built
if command -v npm &> /dev/null && [ -f "web/admin/package.json" ]; then
    echo "Building frontend..."
    cd web/admin
    [ -d "node_modules" ] || npm install
    npm run build
    cd ../..
    cp -r web/admin/dist/* ansible/api-build/web/
elif [ -d "web/admin/dist" ]; then
    echo "Using pre-built frontend..."
    cp -r web/admin/dist/* ansible/api-build/web/
else
    echo "Warning: No frontend available (install Node.js to build)"
fi

# Create Dockerfile
cat > ansible/api-build/Dockerfile << 'EOF'
FROM alpine:latest
RUN apk --no-cache add ca-certificates docker-cli docker-cli-compose
WORKDIR /app
COPY hibana-api ./hibana-api
COPY web/ ./web/
EXPOSE 3000
CMD ["./hibana-api", "--port", "3000", "--static", "./web"]
EOF

echo ""
echo "Setup complete. Run: sudo ./hibana init"
