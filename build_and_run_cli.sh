#!/bin/bash
# This script builds the clangd-query project and then runs the clangd-query tool with all arguments forwarded

set -e

# Change to the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

npm run build
exec node dist/client.js "$@"
