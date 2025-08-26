#!/bin/bash

# Build release binaries for multiple platforms

set -e

echo "Building release binaries..."

# Create releases directory inside bin
mkdir -p bin/releases

# Go to the go directory
cd go

# Build flags for all release builds
LDFLAGS="-s -w"
EXTRA_FLAGS="-trimpath"

# Build for macOS ARM64 (Apple Silicon)
echo "Building for macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build \
    -ldflags="$LDFLAGS" \
    $EXTRA_FLAGS \
    -o ../bin/releases/clangd-query-darwin-arm64 \
    .

# Build for Linux AMD64 (Intel/AMD)
echo "Building for Linux AMD64..."
GOOS=linux GOARCH=amd64 go build \
    -ldflags="$LDFLAGS" \
    $EXTRA_FLAGS \
    -o ../bin/releases/clangd-query-linux-amd64 \
    .

# Show the results
cd ..
echo ""
echo "✓ Build complete! Release binaries:"
ls -lh bin/releases/clangd-query-*