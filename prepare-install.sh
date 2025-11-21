#!/bin/bash

set -e

echo "========================================="
echo "Hibana Stack - Prepare Installation"
echo "========================================="
echo ""
echo "This script prepares the API and admin interface"
echo "BEFORE running 'sudo ./bin/hibana init'"
echo ""

# Check if we're in the right directory
if [ ! -f "build-all.sh" ]; then
    echo "❌ Error: Run this script from the hibana-stack root directory"
    exit 1
fi

# Build everything
echo "🔨 Building API and frontend..."
echo ""

# Build Go binaries
echo "→ Building Go binaries..."
go build -o bin/hibana cmd/hibana/main.go
go build -o bin/hibana-api cmd/api/main.go
echo "  ✓ Go binaries built"

# Check if Node.js is available
if ! command -v npm &> /dev/null; then
    echo ""
    echo "⚠️  Node.js/npm not found!"
    echo ""
    echo "You have two options:"
    echo ""
    echo "1. Install Node.js now:"
    echo "   curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -"
    echo "   sudo apt-get install -y nodejs"
    echo ""
    echo "2. Skip frontend build (placeholder API will be used)"
    echo ""
    read -p "Install Node.js now? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "→ Installing Node.js..."
        curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
        sudo apt-get install -y nodejs
    else
        echo "⚠️  Skipping frontend build"
        echo "   The installation will use a placeholder API"
        echo "   You can build the frontend later with: ./build-all.sh"
        exit 0
    fi
fi

# Build React frontend
echo ""
echo "→ Building React admin interface..."
cd web/admin

if [ ! -d "node_modules" ]; then
    echo "  → Installing npm dependencies..."
    npm install
fi

echo "  → Building React app..."
npm run build

cd ../..

echo ""
echo "========================================="
echo "✅ Build completed successfully!"
echo "========================================="
echo ""
echo "You can now run the installation:"
echo "  sudo ./bin/hibana init"
echo ""
