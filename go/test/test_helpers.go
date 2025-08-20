package test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestContext holds the context for running tests, including paths and utilities
// for executing the clangd-query binary and asserting on its output.
type TestContext struct {
	// BinaryPath is the path to the clangd-query binary that will be tested.
	BinaryPath string
	
	// SampleProjectPath is the path to the C++ test fixture project that
	// provides a known codebase for testing clangd-query commands.
	SampleProjectPath string
	
	// T is the testing context from Go's testing package.
	T *testing.T
}

// NewTestContext creates a new test context with proper paths set up.
// It determines the paths relative to the test file location and ensures
// the binary exists before tests run.
func NewTestContext(t *testing.T) *TestContext {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get current file path")
	}
	
	// test_helpers.go is in go/test/, we need to go up to project root
	testDir := filepath.Dir(filename)
	goDir := filepath.Dir(testDir)
	projectRoot := filepath.Dir(goDir)
	
	tc := &TestContext{
		BinaryPath:        filepath.Join(projectRoot, "bin", "clangd-query"),
		SampleProjectPath: filepath.Join(projectRoot, "test", "fixtures", "sample-project"),
		T:                 t,
	}
	
	// Verify the binary exists
	if _, err := os.Stat(tc.BinaryPath); os.IsNotExist(err) {
		t.Fatalf("clangd-query binary not found at %s. Run ./build.sh first.", tc.BinaryPath)
	}
	
	// Verify the sample project exists
	if _, err := os.Stat(tc.SampleProjectPath); os.IsNotExist(err) {
		t.Fatalf("Sample project not found at %s", tc.SampleProjectPath)
	}
	
	return tc
}

// CommandResult holds the output from running a clangd-query command.
type CommandResult struct {
	// Stdout contains the standard output from the command.
	Stdout string
	
	// Stderr contains the standard error output from the command.
	Stderr string
	
	// ExitCode is the exit code returned by the command.
	ExitCode int
}

// RunCommand executes the clangd-query binary with the given arguments and returns
// the result. The command is run with the sample project as the working directory
// and a 30-second timeout to prevent hanging tests.
func (tc *TestContext) RunCommand(args ...string) *CommandResult {
	cmd := exec.Command(tc.BinaryPath, args...)
	cmd.Dir = tc.SampleProjectPath
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	// Start the command with a timeout
	err := cmd.Start()
	if err != nil {
		tc.T.Fatalf("Failed to start command: %v", err)
	}
	
	// Wait for completion with timeout
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				tc.T.Fatalf("Command failed: %v", err)
			}
		}
		return &CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		}
	case <-time.After(30 * time.Second):
		cmd.Process.Kill()
		tc.T.Fatal("Command timed out after 30 seconds")
		return nil
	}
}

// WaitForDaemonReady ensures the daemon is ready by running a status command.
// This is important for the first test to ensure the daemon has started and
// indexed the codebase before running actual test commands.
func (tc *TestContext) WaitForDaemonReady() {
	// Give the daemon extra time to start up and index
	result := tc.RunCommandWithTimeout([]string{"status"}, 60*time.Second)
	if result.ExitCode != 0 {
		tc.T.Fatalf("Daemon not ready: %s", result.Stderr)
	}
}

// RunCommandWithTimeout is like RunCommand but allows specifying a custom timeout.
// This is useful for commands that may take longer, like initial daemon startup.
func (tc *TestContext) RunCommandWithTimeout(args []string, timeout time.Duration) *CommandResult {
	cmd := exec.Command(tc.BinaryPath, args...)
	cmd.Dir = tc.SampleProjectPath
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Start()
	if err != nil {
		tc.T.Fatalf("Failed to start command: %v", err)
	}
	
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case err := <-done:
		exitCode := 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				tc.T.Fatalf("Command failed: %v", err)
			}
		}
		return &CommandResult{
			Stdout:   stdout.String(),
			Stderr:   stderr.String(),
			ExitCode: exitCode,
		}
	case <-time.After(timeout):
		cmd.Process.Kill()
		tc.T.Fatalf("Command timed out after %v", timeout)
		return nil
	}
}

// AssertExitCode checks that the command exited with the expected code.
func (tc *TestContext) AssertExitCode(result *CommandResult, expected int) {
	if result.ExitCode != expected {
		tc.T.Errorf("Expected exit code %d, got %d\nStdout: %s\nStderr: %s",
			expected, result.ExitCode, result.Stdout, result.Stderr)
	}
}

// AssertContains checks that the output contains the expected string.
func (tc *TestContext) AssertContains(output, expected string) {
	if !strings.Contains(output, expected) {
		tc.T.Errorf("Expected output to contain:\n%s\n\nActual output:\n%s",
			expected, output)
	}
}

// AssertNotContains checks that the output does not contain the given string.
func (tc *TestContext) AssertNotContains(output, unexpected string) {
	if strings.Contains(output, unexpected) {
		tc.T.Errorf("Expected output NOT to contain:\n%s\n\nActual output:\n%s",
			unexpected, output)
	}
}

// CountOccurrences returns the number of times substr appears in str.
func CountOccurrences(str, substr string) int {
	return strings.Count(str, substr)
}

// ShutdownDaemon attempts to cleanly shut down the daemon after tests.
// Errors during shutdown are ignored as the daemon may already be stopped.
func (tc *TestContext) ShutdownDaemon() {
	// Ignore errors as daemon might already be stopped
	_ = tc.RunCommand("shutdown")
}