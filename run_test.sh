#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Simple: if argument provided, it's a test filter
TEST_FILTER="$1"

# Always build first
echo "Building clangd-query..."
./build.sh

echo "Running tests..."

# Fix Go environment
unset GOROOT
export GOROOT=

# Run tests
cd go
if [ -n "$TEST_FILTER" ]; then
    echo "Running: $TEST_FILTER"
    # Run tests in both test directories
    go test -v ./test -run "$TEST_FILTER"
    go test -v ./internal/lsp -run "$TEST_FILTER"
else
    echo "Running all tests"
    # Run tests in both test directories
    go test -v ./test ./internal/lsp
fi