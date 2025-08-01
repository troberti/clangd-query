#!/bin/bash

# Simple wrapper to run clangd-query on the sample project
# Usage: ./query.sh <command> [args...]

cd "$(dirname "$0")/fixtures/sample-project"
exec ../../../bin/clangd-query "$@"