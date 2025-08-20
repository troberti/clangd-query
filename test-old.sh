#!/bin/bash

# Test script for clangd-query TypeScript (original) implementation
# Runs commands against the sample project

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Paths
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
CLIENT_JS="$SCRIPT_DIR/dist/client.js"
SAMPLE_PROJECT="$SCRIPT_DIR/test/fixtures/sample-project"

# Check if Node.js is available
if ! command -v node &> /dev/null; then
    echo -e "${RED}Error: Node.js not found${NC}"
    echo "Please install Node.js to run the TypeScript implementation"
    exit 1
fi

# Check if TypeScript client exists
if [ ! -f "$CLIENT_JS" ]; then
    echo -e "${RED}Error: TypeScript client not found at $CLIENT_JS${NC}"
    echo "Run 'npm run build' first to compile TypeScript"
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
    echo -e "${YELLOW}CMD:${NC} clangd-query (TypeScript) $2"
    echo "----------------------------------------"
}

# Function to run a command in the sample project
run_test() {
    local description="$1"
    shift
    print_test "$description" "$*"
    (cd "$SAMPLE_PROJECT" && node "$CLIENT_JS" "$@")
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
    echo -e "${GREEN}Running clangd-query TypeScript (original) implementation tests${NC}"
    echo "Client: $CLIENT_JS"
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
    (cd "$SAMPLE_PROJECT" && node "$CLIENT_JS" shutdown)

else
    # Run specific command passed as arguments
    echo -e "${GREEN}Running custom command (TypeScript)${NC}"
    echo "Working directory: $SAMPLE_PROJECT"
    (cd "$SAMPLE_PROJECT" && node "$CLIENT_JS" "$@")
fi