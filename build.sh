#!/bin/bash

# Simple build script for clangd-query

cd go
mkdir -p ../bin

if go build -o ../bin/clangd-query .; then
    echo "✓ Build successful: bin/clangd-query"
else
    echo "✗ Build failed"
    exit 1
fi