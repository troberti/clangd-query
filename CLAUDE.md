# CLAUDE.md - clangd-query Development Guidelines

This file provides guidance to Claude Code when working on the clangd-query project.

## Core Principles
- Keep the Go code simple, idiomatic, and maintainable
- Always use the logger for observability
- Maintain consistency with the existing codebase
- Write comprehensive documentation in prose, not terse comments

## Daemon Development Rules

### Logging is MANDATORY
- In the daemon code, ALWAYS use the logger for ALL messages (errors, warnings, info)
- Never return errors without logging them first
- This ensures all issues are visible in the daemon logs for debugging

Example:
```go
// WRONG - error not logged
if input == "" {
    return nil, fmt.Errorf("search requires an input parameter")
}

// CORRECT - error is logged before returning
if input == "" {
    d.logger.Error("search requires an input parameter")
    return nil, fmt.Errorf("search requires an input parameter")
}
```

## Go Code Documentation Guidelines

Write comprehensive documentation for all Go code using proper prose, not terse comments. Go developers value clear, readable documentation that explains not just what the code does, but why it exists and how to use it effectively.

### Documentation Style
- Write documentation in complete sentences with proper grammar and punctuation
- Explain the purpose and behavior of types, functions, and fields in detail
- Include examples and edge cases where relevant
- Document assumptions, invariants, and important relationships between components
- Use multiple paragraphs when needed to fully explain complex concepts

### Example of Good Documentation
```go
// Plain data struct to represents structured documentation extracted from clangd's hover response.
// This struct provides a consistent interface for accessing various pieces of documentation
// without needing to parse raw markdown in individual commands. All parsing logic should be
// centralized in the GetDocumentation method to ensure consistency across the codebase.
type ParsedDocumentation struct {
    // The cleaned documentation text without technical details like size/offset/alignment
    // information. Contains the human-readable documentation that explains what a symbol
    // does, typically extracted from doc comments like @brief or plain documentation text.
    // Line breaks and formatting are preserved where meaningful.
    Description string
}

// A simple struct to hold the context for running tests, including paths and
// utilities for executing the clangd-query binary and asserting on its output.
type TestContext struct {
    // Path to the clangd-query binary that will be tested.
    BinaryPath string

    // Path to the C++ test fixture project that provides a known codebase
    // for testing clangd-query commands.
    SampleProjectPath string

    // The testing context from Go's testing package.
    T *testing.T
}
```

IMPORTANT: AVOID comments like this where the class or function name is the start
of the sentence. It just reads awkward.
```go
// TestComplexRealWorldExamples tests complex real-world hover responses
func TestComplexRealWorldExamples(t *testing.T) {
  ...  // ^^ BAD
}
```
Instead write it like this:
```go
// Tests complex real-world hover responses
func TestComplexRealWorldExamples(t *testing.T) {
  ...  // ^^ GOOD
}
```


### Avoid Terse Comments
Don't write minimal comments like:
```go
// Description is the description  // BAD - says nothing useful
Description string

// Returns the value  // BAD - obvious from signature
func GetValue() int
```

## Code Organization
- `internal/client/` - Client-side code for CLI interactions
- `internal/daemon/` - Daemon process that manages clangd
- `internal/lsp/` - LSP protocol implementation for clangd communication
- `internal/commands/` - Command implementations
- `internal/logger/` - Logging interface and implementations

## Code Formatting
- All Go code MUST be formatted with `gofmt` before committing
- Run `./format_go.sh` to automatically format all Go source files
- The formatting script will tell you if any files were modified
- This ensures consistent code style across the entire Go codebase

## CRITICAL: Working Directory Rules
**YOU MUST ALWAYS STAY IN THE PROJECT ROOT DIRECTORY!**
- NEVER use `cd` to change to subdirectories
- ALWAYS run commands from the root directory.
- USE THE PROVIDED SCRIPTS:
  - `./test.sh <command>` - Run commands on the test fixture
  - `./compare.sh <command>` - Compare Go vs TypeScript implementations
  - `./test.sh logs --verbose` - View verbose daemon logs for debugging
  - `./build.sh` - Build the Go implementation

## Testing & Debugging
- Always test commands after making changes
- Use the test fixture project in `test/fixtures/sample-project` via the scripts
- Verify both successful operations and error cases
- For debugging: Use the logger and examine logs with `./test.sh logs --verbose`
- NEVER create debug files or test programs

## Common Commands for Testing
```bash
# Format Go code before committing
./format_go.sh

# Build the project
./build.sh

# Test basic functionality in the example testing code base. This runs
# bin/clangd-query but with the working directory of test/fixtures/sample-project
./test.sh search GameObject
./test.sh show GameObject
./test.sh logs --verbose
```

## Error Handling
- Log errors at the point they occur
- Return meaningful error messages to the client
- Use structured logging where possible

## Performance Considerations
- The daemon should start quickly and cache clangd connections
- Use in-memory log buffers to avoid excessive file I/O
- Implement timeouts for all network operations


## Development

Use `./test.sh` to run the clangd-query tool in Go against the test source
database. Use `./test-old.sh` to use the old Typescript version.

Use `./compare.sh` to run both versions and compare their output.

ONLY use these tools when developing.

## Testing

Run `./run_tests.sh` to run all Go tests. To run a specific test, add a
filter argument, like `./run_test.sh TestInterfaceCommand`.