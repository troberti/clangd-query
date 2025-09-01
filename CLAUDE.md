# CLAUDE.md - clangd-query Development Guidelines

This file provides guidance to Claude Code when working on the clangd-query project.

## Core Principles
- Keep the Go code simple, idiomatic, and maintainable
- Always use the logger for observability
- Maintain consistency with the existing codebase
- Write comprehensive documentation in prose, not terse comments

## Daemon Development Rules

### Logging is MANDATORY
In the daemon code, ALWAYS use the logger for ALL messages (errors, warnings, info)
instead of stdin or stderr

EVEYR ERROR THAT IS HANDLED MUST BE LOGGED! There cannot be any errors that are
discarded without log messages anywhere in the daemon process.

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
    // does.
    //
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

## Example of Bad Documentation

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

Also NEVER write these redundant implementation comments that are essentially
repeating the code. Use implementation comments to notify the user of critical
requirements and why a specific implementation is chosen. Essentially, try to
answer the questions that a proficient reader of programming languages would have
when reading your code. So do not do this:
```go
func formatLocation(client *lsp.ClangdClient, location lsp.Location) string {
	// Extract path from URI
	absolutePath := client.PathFromFileURI(location.URI)

	// Make path relative
	relativePath := client.ToRelativePath(absolutePath)

	// Format with 1-based line and column numbers
	return fmt.Sprintf("%s:%d:%d", relativePath,
		location.Range.Start.Line+1,
		location.Range.Start.Character+1)
}
```

But structure it like so:
```go
// Formats the path of the `location` as a "file:line:column" string. The column
// value is 1-based.
func formatLocation(client *lsp.ClangdClient, location lsp.Location) string {
	absolutePath := client.PathFromFileURI(location.URI)
	relativePath := client.ToRelativePath(absolutePath)
	return fmt.Sprintf("%s:%d:%d", relativePath,
		location.Range.Start.Line+1,
		location.Range.Start.Character+1)
}
```


## Code Organization
- `internal/client/` - Client-side code for CLI interactions
- `internal/daemon/` - Daemon process that manages clangd
- `internal/clangd/` - Classes to interact with the clangd process
- `internal/commands/` - Command implementations, includes output formatting
- `internal/logger/` - Logging interface and implementations


## Code Formatting
Run `./format_go.sh` to automatically format all Go source files use gofmt.
The formatting script will tell you if any files were modified.

## CRITICAL: Working Directory Rules
**YOU MUST ALWAYS STAY IN THE PROJECT ROOT DIRECTORY!**
- NEVER use `cd` to change to subdirectories
- ALWAYS run commands from the root directory.
- USE THE PROVIDED SCRIPTS:
  - `./test.sh <command>` - Run commands on the test fixture
  - `./test.sh logs --verbose` - View verbose daemon logs for debugging
  - `./build.sh` - Build the Go implementation

## Testing & Debugging
- Always test commands after making changes
- Use the test fixture project in `test/fixtures/sample-project` via the `./test.sh`
  script.
- Verify both successful operations and error cases
- For debugging: Use the logger and examine logs with `./test.sh logs --verbose`
- NEVER create debug files or test programs

## Common Commands for Development
```bash
# Format Go code before committing
./format_go.sh

# Build the binary in bin/clangd-query
./build.sh

# Test basic functionality in the example testing code base. This runs
# bin/clangd-query but with the working directory of test/fixtures/sample-project
./test.sh search GameObject
./test.sh show GameObject
./test.sh logs --verbose
```

## Testing

Run `./run_tests.sh` to run all Go tests. To run a specific test, add a
filter argument, like `./run_test.sh TestInterfaceCommand`.