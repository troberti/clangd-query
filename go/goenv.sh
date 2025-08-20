#!/bin/bash

# Go environment setup script
# Source this file or run commands through it

# Set up Go environment
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org

if [ $# -eq 0 ]; then
    # No arguments - just print the environment
    echo "Go environment configured:"
    echo "  GOROOT=$GOROOT"
    echo "  GOPATH=$GOPATH"
    echo "  GOPROXY=$GOPROXY"
    echo "  GOSUMDB=$GOSUMDB"
    echo ""
    echo "Usage:"
    echo "  source goenv.sh           # Set environment in current shell"
    echo "  ./goenv.sh <command>      # Run command with Go environment"
    echo ""
    echo "Examples:"
    echo "  ./goenv.sh go build"
    echo "  ./goenv.sh go test ./..."
else
    # Run the command with the environment
    exec "$@"
fi