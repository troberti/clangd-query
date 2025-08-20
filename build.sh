#!/bin/bash

cd go

# Set up Go environment
export GOROOT=/usr/local/go
export GOPATH=$HOME/go
export PATH=$GOROOT/bin:$GOPATH/bin:$PATH
export GOPROXY=https://proxy.golang.org
export GOSUMDB=sum.golang.org

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[+]${NC} $1"
}

print_error() {
    echo -e "${RED}[!]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[*]${NC} $1"
}

# Check if we're in the right directory
if [ ! -f "go.mod" ]; then
    print_error "Error: go.mod not found. Please run this script from the go/ directory"
    exit 1
fi

# Parse command line arguments
COMMAND=${1:-build}
OUTPUT_DIR="../bin"
OUTPUT_NAME="clangd-query"

case $COMMAND in
    build)
        print_status "Building clangd-query..."
        mkdir -p "$OUTPUT_DIR"
        if go build -o "$OUTPUT_DIR/$OUTPUT_NAME" .; then
            print_status "Build successful! Binary created at $OUTPUT_DIR/$OUTPUT_NAME"
            ls -lh "$OUTPUT_DIR/$OUTPUT_NAME"
        else
            print_error "Build failed"
            exit 1
        fi
        ;;

    test)
        print_status "Running tests..."
        go test ./...
        ;;

    run)
        shift # Remove 'run' from arguments
        print_status "Building and running clangd-query..."
        if go build -o "$OUTPUT_DIR/$OUTPUT_NAME" .; then
            "$OUTPUT_DIR/$OUTPUT_NAME" "$@"
        else
            print_error "Build failed"
            exit 1
        fi
        ;;

    fmt)
        print_status "Formatting code..."
        go fmt ./...
        ;;

    vet)
        print_status "Running go vet..."
        go vet ./...
        ;;

    deps)
        print_status "Downloading dependencies..."
        go mod download
        go mod tidy
        ;;

    clean)
        print_status "Cleaning build artifacts..."
        rm -f "$OUTPUT_DIR/$OUTPUT_NAME"
        go clean
        print_status "Clean complete"
        ;;

    install)
        print_status "Installing clangd-query to /usr/local/bin..."
        if go build -o "$OUTPUT_DIR/$OUTPUT_NAME" .; then
            if [ -w "/usr/local/bin" ]; then
                cp "$OUTPUT_DIR/$OUTPUT_NAME" /usr/local/bin/
                print_status "Installed to /usr/local/bin/clangd-query"
            else
                print_warning "Need sudo to install to /usr/local/bin"
                sudo cp "$OUTPUT_DIR/$OUTPUT_NAME" /usr/local/bin/
                print_status "Installed to /usr/local/bin/clangd-query"
            fi
        else
            print_error "Build failed"
            exit 1
        fi
        ;;

    help|--help|-h)
        echo "Usage: ./build.sh [command] [args...]"
        echo ""
        echo "Commands:"
        echo "  build     - Build the binary (default)"
        echo "  test      - Run tests"
        echo "  run       - Build and run with arguments"
        echo "  fmt       - Format code"
        echo "  vet       - Run go vet"
        echo "  deps      - Download and tidy dependencies"
        echo "  clean     - Clean build artifacts"
        echo "  install   - Build and install to /usr/local/bin"
        echo "  help      - Show this help message"
        echo ""
        echo "Examples:"
        echo "  ./build.sh                    # Build the binary"
        echo "  ./build.sh run search Widget  # Build and search for 'Widget'"
        echo "  ./build.sh test              # Run tests"
        ;;

    *)
        print_error "Unknown command: $COMMAND"
        echo "Run './build.sh help' for usage information"
        exit 1
        ;;
esac