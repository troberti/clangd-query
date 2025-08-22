#!/bin/bash

# Format all Go source files in the clangd-query Go implementation
# This ensures consistent code style across the codebase

set -e
echo "Formatting Go source files..."
# Format all Go files in the go/ directory
gofmt -w go/
echo "✓ Go formatting complete"
