#!/bin/bash

# Compare outputs between Go and TypeScript implementations
# Usage: ./compare.sh <command> [args...]
# Example: ./compare.sh search Widget

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Check arguments
if [ $# -eq 0 ]; then
    echo -e "${RED}Error: No command specified${NC}"
    echo "Usage: $0 <command> [args...]"
    echo "Examples:"
    echo "  $0 search Widget"
    echo "  $0 show GameScene"
    echo "  $0 view Button"
    exit 1
fi

# Paths
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
SAMPLE_PROJECT="$SCRIPT_DIR/test/fixtures/sample-project"

echo -e "${CYAN}Comparing Go vs TypeScript implementations${NC}"
echo -e "${YELLOW}Command:${NC} clangd-query $*"
echo "Project directory: $SAMPLE_PROJECT"
echo ""

# Create temporary files for outputs
GO_OUTPUT=$(mktemp)
TS_OUTPUT=$(mktemp)

# Function to cleanup temp files
cleanup() {
    rm -f "$GO_OUTPUT" "$TS_OUTPUT"
}
trap cleanup EXIT

# Run Go implementation
echo -e "${BLUE}=== Go Implementation Output ===${NC}"
(cd "$SAMPLE_PROJECT" && "$SCRIPT_DIR/bin/clangd-query" "$@" 2>&1) | tee "$GO_OUTPUT"
GO_EXIT_CODE=${PIPESTATUS[0]}

echo ""

# Run TypeScript implementation
echo -e "${BLUE}=== TypeScript Implementation Output ===${NC}"
(cd "$SAMPLE_PROJECT" && node "$SCRIPT_DIR/dist/client.js" "$@" 2>&1) | tee "$TS_OUTPUT"
TS_EXIT_CODE=${PIPESTATUS[0]}

echo ""

# Compare exit codes
echo -e "${BLUE}=== Comparison Summary ===${NC}"
if [ $GO_EXIT_CODE -eq $TS_EXIT_CODE ]; then
    echo -e "${GREEN}✓ Exit codes match:${NC} $GO_EXIT_CODE"
else
    echo -e "${RED}✗ Exit codes differ:${NC} Go=$GO_EXIT_CODE, TypeScript=$TS_EXIT_CODE"
fi

# Compare outputs
if cmp -s "$GO_OUTPUT" "$TS_OUTPUT"; then
    echo -e "${GREEN}✓ Outputs are identical${NC}"
else
    echo -e "${YELLOW}⚠ Outputs differ${NC}"
    echo ""
    echo -e "${BLUE}=== Differences (Go vs TypeScript) ===${NC}"
    # Use diff with color if available
    if command -v colordiff &> /dev/null; then
        colordiff -u "$GO_OUTPUT" "$TS_OUTPUT" || true
    else
        diff -u "$GO_OUTPUT" "$TS_OUTPUT" || true
    fi
fi
