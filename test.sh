#!/bin/bash

# Run clangd-query in the test fixture directory

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Run in test fixture directory
cd "$SCRIPT_DIR/test/fixtures/sample-project"
"$SCRIPT_DIR/bin/clangd-query" "$@"