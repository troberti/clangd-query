package test

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestMain is the entry point for the test suite. It sets up the global test
// context, starts the daemon once, and runs all tests with the shared daemon.
func TestMain(m *testing.M) {
	log.Println("TestMain: Starting test suite setup...")
	
	// Initialize the global test context
	if err := initializeGlobalContext(); err != nil {
		log.Fatalf("Failed to initialize test context: %v", err)
	}
	
	// Ensure daemon is ready before running any tests
	log.Println("Starting daemon and waiting for it to be ready...")
	if err := waitForDaemonReady(); err != nil {
		log.Fatalf("Failed to start daemon: %v", err)
	}
	log.Println("Daemon is ready, running tests...")
	
	// Run all tests
	exitCode := m.Run()
	
	// Always shutdown daemon before exiting
	log.Println("Shutting down daemon...")
	shutdownDaemon()
	
	os.Exit(exitCode)
}

// initializeGlobalContext sets up the global test context with paths.
func initializeGlobalContext() error {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return os.ErrNotExist
	}
	
	// main_test.go is in go/test/, we need to go up to project root
	testDir := filepath.Dir(filename)
	goDir := filepath.Dir(testDir)
	projectRoot := filepath.Dir(goDir)
	
	globalTestContext = &TestContext{
		BinaryPath:        filepath.Join(projectRoot, "bin", "clangd-query"),
		SampleProjectPath: filepath.Join(projectRoot, "test", "fixtures", "sample-project"),
	}
	
	// Verify the binary exists
	if _, err := os.Stat(globalTestContext.BinaryPath); os.IsNotExist(err) {
		return err
	}
	
	// Verify the sample project exists
	if _, err := os.Stat(globalTestContext.SampleProjectPath); os.IsNotExist(err) {
		return err
	}
	
	return nil
}

// waitForDaemonReady ensures the daemon is ready by running a status command.
func waitForDaemonReady() error {
	cmd := exec.Command(globalTestContext.BinaryPath, "status")
	cmd.Dir = globalTestContext.SampleProjectPath
	
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	err := cmd.Start()
	if err != nil {
		return err
	}
	
	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()
	
	select {
	case err := <-done:
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				if exitErr.ExitCode() != 0 {
					return exitErr
				}
			}
			return err
		}
		return nil
	case <-time.After(60 * time.Second):
		cmd.Process.Kill()
		return os.ErrDeadlineExceeded
	}
}

// shutdownDaemon cleanly shuts down the daemon after tests complete.
func shutdownDaemon() {
	cmd := exec.Command(globalTestContext.BinaryPath, "shutdown")
	cmd.Dir = globalTestContext.SampleProjectPath
	
	// We don't care too much about errors here, just try to shut it down
	_ = cmd.Run()
}