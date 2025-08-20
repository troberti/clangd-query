#!/bin/bash

# Run clangd-query (TypeScript version) in the test fixture directory

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Run in test fixture directory
cd "$SCRIPT_DIR/test/fixtures/sample-project"
node "$SCRIPT_DIR/dist/client.js" "$@"