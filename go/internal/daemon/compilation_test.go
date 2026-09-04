package daemon

import (
	"os"
	"path/filepath"
	"testing"

	"clangd-query/internal/logger"
)

func TestExpandCMakeVariables(t *testing.T) {
	projectRoot := filepath.Clean("/project/root")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "sourceDir and presetName",
			input: "${sourceDir}/build/${presetName}",
			want:  filepath.Join(projectRoot, "build", "flexspi_nor_debug"),
		},
		{
			name:  "fileDir and presetName",
			input: "${fileDir}/${presetName}",
			want:  filepath.Join(projectRoot, "flexspi_nor_debug"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := expandCMakeVariables(tc.input, projectRoot, "flexspi_nor_debug")
			if got != tc.want {
				t.Fatalf("expandCMakeVariables() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCMakePresetBuildDirs(t *testing.T) {
	dir := t.TempDir()
	presets := `{
  "configurePresets": [
    {
      "name": "base",
      "binaryDir": "${sourceDir}/build/base"
    },
    {
      "name": "flexspi_nor_debug",
      "inherits": "base",
      "binaryDir": "${sourceDir}/build/flexspi_nor_debug"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "CMakePresets.json"), []byte(presets), 0644); err != nil {
		t.Fatalf("failed to write presets file: %v", err)
	}

	dirs, err := cmakePresetBuildDirs(dir)
	if err != nil {
		t.Fatalf("cmakePresetBuildDirs() error: %v", err)
	}

	if len(dirs) != 2 {
		t.Fatalf("cmakePresetBuildDirs() returned %d dirs, want 2: %v", len(dirs), dirs)
	}
}

func TestFindExistingCompileCommandsPrefersPresetBuildDir(t *testing.T) {
	dir := t.TempDir()
	presetBuildDir := filepath.Join(dir, "build", "flexspi_nor_debug")
	if err := os.MkdirAll(presetBuildDir, 0755); err != nil {
		t.Fatalf("failed to create preset build dir: %v", err)
	}

	compileCommands := `[{"directory":"/tmp","file":"main.cpp","command":"clang++ main.cpp"}]`
	if err := os.WriteFile(filepath.Join(presetBuildDir, "compile_commands.json"), []byte(compileCommands), 0644); err != nil {
		t.Fatalf("failed to write compile_commands.json: %v", err)
	}

	presets := `{
  "configurePresets": [
    {
      "name": "flexspi_nor_debug",
      "binaryDir": "${sourceDir}/build/flexspi_nor_debug"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "CMakePresets.json"), []byte(presets), 0644); err != nil {
		t.Fatalf("failed to write presets file: %v", err)
	}

	found, err := findExistingCompileCommands(dir, testLogger{t})
	if err != nil {
		t.Fatalf("findExistingCompileCommands() error: %v", err)
	}
	if found != presetBuildDir {
		t.Fatalf("findExistingCompileCommands() = %q, want %q", found, presetBuildDir)
	}
}

func TestFindExistingCompileCommandsResolvesFileDirPreset(t *testing.T) {
	dir := t.TempDir()
	presetBuildDir := filepath.Join(dir, "flexspi_nor_debug")
	if err := os.MkdirAll(presetBuildDir, 0755); err != nil {
		t.Fatalf("failed to create preset build dir: %v", err)
	}

	compileCommands := `[{"directory":"/tmp","file":"main.cpp","command":"clang++ main.cpp"}]`
	if err := os.WriteFile(filepath.Join(presetBuildDir, "compile_commands.json"), []byte(compileCommands), 0644); err != nil {
		t.Fatalf("failed to write compile_commands.json: %v", err)
	}

	presets := `{
  "configurePresets": [
    {
      "name": "flexspi_nor_debug",
      "binaryDir": "${fileDir}/${presetName}"
    }
  ]
}`
	if err := os.WriteFile(filepath.Join(dir, "CMakePresets.json"), []byte(presets), 0644); err != nil {
		t.Fatalf("failed to write presets file: %v", err)
	}

	found, err := findExistingCompileCommands(dir, testLogger{t})
	if err != nil {
		t.Fatalf("findExistingCompileCommands() error: %v", err)
	}
	if found != presetBuildDir {
		t.Fatalf("findExistingCompileCommands() = %q, want %q", found, presetBuildDir)
	}
}

type testLogger struct {
	t *testing.T
}

func (l testLogger) Debug(format string, args ...interface{}) {}
func (l testLogger) Info(format string, args ...interface{})  {}
func (l testLogger) Error(format string, args ...interface{}) {}

func (l testLogger) GetLogs(minLevel logger.LogLevel) string { return "" }
