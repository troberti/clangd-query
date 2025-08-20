#!/bin/bash

# Run clangd-query (Go version) in the test fixture directory

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Fix Go environment
unset GOROOT
export GOROOT=

# Run in test fixture directory
cd "$SCRIPT_DIR/test/fixtures/sample-project"
"$SCRIPT_DIR/bin/clangd-query" "$@"