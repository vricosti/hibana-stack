#!/bin/bash

set -e

echo "========================================="
echo "Building Hibana Stack - Full Build"
echo "========================================="

# Build Go binaries
echo ""
echo "📦 Building Go binaries..."
echo ""

# Build hibana CLI
echo "Building hibana CLI..."
go build -o bin/hibana cmd/hibana/main.go
echo "✓ hibana CLI built: bin/hibana"

# Build hibana API
echo "Building hibana API..."
go build -o bin/hibana-api cmd/api/main.go
echo "✓ hibana API built: bin/hibana-api"

# Build React admin interface
echo ""
echo "🌐 Building React admin interface..."
echo ""

cd web/admin

# Check if node_modules exists
if [ ! -d "node_modules" ]; then
    echo "Installing npm dependencies..."
    npm install
fi

echo "Building React app..."
npm run build

cd ../..

echo ""
echo "========================================="
echo "✅ Build completed successfully!"
echo "========================================="
echo ""
echo "Binaries:"
echo "  - bin/hibana          (CLI installer)"
echo "  - bin/hibana-api      (API server)"
echo ""
echo "Frontend:"
echo "  - web/admin/dist/     (React admin interface)"
echo ""
echo "To start the API server:"
echo "  sudo ./bin/hibana-api --port 3000 --static ./web/admin/dist"
echo ""
