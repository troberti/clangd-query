#!/bin/bash

# Run Go tests for clangd-query

cd go

if [ -n "$1" ]; then
    # Run specific test
    go test -v ./test ./internal/lsp -run "$1"
else
    # Run all tests
    go test -v ./test ./internal/lsp
fi