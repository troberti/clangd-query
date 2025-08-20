package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"clangd-query/internal/logger"
)

// EnsureCompilationDatabase ensures compile_commands.json exists in the build directory
func EnsureCompilationDatabase(projectRoot string, log logger.Logger) (string, error) {
	buildDir := filepath.Join(projectRoot, ".cache", "clangd-query", "build")
	compileCommandsPath := filepath.Join(buildDir, "compile_commands.json")

	// Check if it already exists
	if _, err := os.Stat(compileCommandsPath); err == nil {
		return buildDir, nil
	}

	// Create build directory
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create build directory: %v", err)
	}

	// Run CMake to generate compile_commands.json
	log.Info("Generating compile_commands.json in %s...", buildDir)

	cmd := exec.Command("cmake",
		"-S", projectRoot,
		"-B", buildDir,
		"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON")

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if cmake is not found
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("cmake not found in PATH. Please install CMake to use clangd-query")
		}
		return "", fmt.Errorf("cmake failed: %v\nOutput: %s", err, output)
	}

	// Verify the file was created
	if _, err := os.Stat(compileCommandsPath); err != nil {
		return "", fmt.Errorf("cmake succeeded but compile_commands.json was not created")
	}

	return buildDir, nil
}
