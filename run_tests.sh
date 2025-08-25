#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# If argument provided, it's a test filter
TEST_FILTER="$1"

echo "Running tests..."
cd go
if [ -n "$TEST_FILTER" ]; then
    echo "Running: $TEST_FILTER"
    go test -v ./test -run "$TEST_FILTER"
    go test -v ./internal/lsp -run "$TEST_FILTER"
else
    # Run tests in all test directories
    echo "Running all tests"
    go test -v ./test ./internal/lsp
fi