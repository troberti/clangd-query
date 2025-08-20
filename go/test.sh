#!/bin/bash

# Test script for clangd-query Go implementation
# Runs commands against the sample project

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Paths
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BINARY="$SCRIPT_DIR/../bin/clangd-query"
SAMPLE_PROJECT="$SCRIPT_DIR/../test/fixtures/sample-project"

# Check if binary exists
if [ ! -f "$BINARY" ]; then
    echo -e "${RED}Error: Binary not found at $BINARY${NC}"
    echo "Run './build.sh build' first"
    exit 1
fi

# Check if sample project exists
if [ ! -d "$SAMPLE_PROJECT" ]; then
    echo -e "${RED}Error: Sample project not found at $SAMPLE_PROJECT${NC}"
    exit 1
fi

# Function to print test header
print_test() {
    echo -e "\n${BLUE}TEST:${NC} $1"
    echo -e "${YELLOW}CMD:${NC} clangd-query $2"
    echo "----------------------------------------"
}

# Function to run a command in the sample project
run_test() {
    local description="$1"
    shift
    print_test "$description" "$*"
    (cd "$SAMPLE_PROJECT" && "$BINARY" "$@")
    local exit_code=$?
    if [ $exit_code -eq 0 ]; then
        echo -e "${GREEN}✓ Success${NC}"
    else
        echo -e "${RED}✗ Failed with exit code $exit_code${NC}"
    fi
    return $exit_code
}

# Parse command line arguments
if [ $# -eq 0 ]; then
    # Run default test suite
    echo -e "${GREEN}Running clangd-query Go implementation tests${NC}"
    echo "Binary: $BINARY"
    echo "Sample project: $SAMPLE_PROJECT"
    
    # Basic commands
    run_test "Search for Widget" search Widget
    run_test "Search with limit" search Widget --limit 3
    run_test "Show GameScene" show GameScene
    run_test "View Button class" view Button
    run_test "Find usages of update" usages update
    run_test "Show hierarchy of Widget" hierarchy Widget
    run_test "Get signature of handleClick" signature handleClick
    run_test "Get interface of Widget" interface Widget
    
    # Status commands
    run_test "Check daemon status" status
    run_test "Show daemon logs" logs
    
    # Cleanup
    echo -e "\n${YELLOW}Shutting down daemon...${NC}"
    (cd "$SAMPLE_PROJECT" && "$BINARY" shutdown)
    
else
    # Run specific command passed as arguments
    echo -e "${GREEN}Running custom command${NC}"
    echo "Working directory: $SAMPLE_PROJECT"
    (cd "$SAMPLE_PROJECT" && "$BINARY" "$@")
fi