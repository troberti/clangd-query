package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"clangd-query/internal/logger"
)

// CompilationOptions controls how the daemon locates or generates
// compile_commands.json for a project.
type CompilationOptions struct {
	BuildDir    string
	CMakePreset string
}

// Locates or generates a compilation database for the project. Existing
// databases from the user's normal CMake workflow are preferred over creating
// a separate configure step in .cache/clangd-query/build, because many
// projects require toolchain files or presets that a plain cmake invocation
// cannot reproduce.
func EnsureCompilationDatabase(projectRoot string, opts CompilationOptions, log logger.Logger) (string, error) {
	if opts.BuildDir != "" {
		buildDir := filepath.Clean(opts.BuildDir)
		if hasCompileCommands(buildDir) {
			log.Info("Using compilation database from %s (--build-dir)", buildDir)
			return buildDir, nil
		}
		return "", fmt.Errorf(
			"--build-dir is set to %q but %s does not exist",
			buildDir, filepath.Join(buildDir, "compile_commands.json"),
		)
	}

	cacheBuildDir := filepath.Join(projectRoot, ".cache", "clangd-query", "build")
	if hasCompileCommands(cacheBuildDir) {
		return cacheBuildDir, nil
	}

	if buildDir, err := findExistingCompileCommands(projectRoot, log); err != nil {
		return "", err
	} else if buildDir != "" {
		return buildDir, nil
	}

	preset := strings.TrimSpace(opts.CMakePreset)
	if preset == "" {
		names, err := cmakeConfigurePresetNames(projectRoot)
		if err != nil {
			return "", err
		}
		if len(names) == 1 {
			preset = names[0]
			log.Info("Using sole CMake configure preset %q", preset)
		}
	}

	if preset != "" {
		return generateWithCMakePreset(projectRoot, preset, log)
	}

	return generateWithPlainCMake(projectRoot, cacheBuildDir, log)
}

func findExistingCompileCommands(projectRoot string, log logger.Logger) (string, error) {
	candidates := make([]string, 0, 8)

	presetDirs, err := cmakePresetBuildDirs(projectRoot)
	if err != nil {
		return "", err
	}
	candidates = append(candidates, presetDirs...)

	for _, name := range []string{"build", "out", "cmake-build-debug", "cmake-build-release"} {
		candidates = append(candidates, filepath.Join(projectRoot, name))
	}

	seen := make(map[string]bool)
	var bestDir string
	var bestModTime time.Time

	for _, dir := range candidates {
		dir = filepath.Clean(dir)
		if seen[dir] {
			continue
		}
		seen[dir] = true

		compileCommandsPath := filepath.Join(dir, "compile_commands.json")
		info, err := os.Stat(compileCommandsPath)
		if err != nil || info.IsDir() {
			continue
		}
		if !isCompileCommandsFile(compileCommandsPath) {
			continue
		}

		if bestDir == "" || info.ModTime().After(bestModTime) {
			bestDir = dir
			bestModTime = info.ModTime()
		}
	}

	if bestDir != "" {
		log.Info("Using existing compilation database at %s", filepath.Join(bestDir, "compile_commands.json"))
		return bestDir, nil
	}

	if foundDir, err := searchForCompileCommands(projectRoot); err != nil {
		return "", err
	} else if foundDir != "" {
		log.Info("Using existing compilation database at %s", filepath.Join(foundDir, "compile_commands.json"))
		return foundDir, nil
	}

	return "", nil
}

func searchForCompileCommands(projectRoot string) (string, error) {
	var bestDir string
	var bestModTime time.Time

	err := filepath.WalkDir(projectRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.IsDir() {
			base := entry.Name()
			if strings.HasPrefix(base, ".") && path != projectRoot {
				return filepath.SkipDir
			}
			if base == ".cache" && path != filepath.Join(projectRoot, ".cache", "clangd-query", "build") {
				return filepath.SkipDir
			}
			rel, relErr := filepath.Rel(projectRoot, path)
			if relErr == nil && strings.Count(rel, string(os.PathSeparator)) > 4 {
				return filepath.SkipDir
			}
			return nil
		}

		if entry.Name() != "compile_commands.json" {
			return nil
		}

		if !isCompileCommandsFile(path) {
			return nil
		}

		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}

		dir := filepath.Dir(path)
		if bestDir == "" || info.ModTime().After(bestModTime) {
			bestDir = dir
			bestModTime = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return "", err
	}

	return bestDir, nil
}

func generateWithCMakePreset(projectRoot, preset string, log logger.Logger) (string, error) {
	log.Info("Configuring project with CMake preset %q...", preset)

	cmd := exec.Command(
		"cmake",
		"--preset", preset,
		"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
	)
	cmd.Dir = projectRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("cmake not found in PATH. Please install CMake to use clangd-query")
		}
		return "", fmt.Errorf("cmake --preset %s failed: %v\nOutput: %s", preset, err, output)
	}

	buildDir, err := resolvePresetBinaryDirForProject(projectRoot, preset)
	if err != nil {
		return "", err
	}
	if buildDir == "" {
		return "", fmt.Errorf("cmake preset %q succeeded but its binaryDir could not be determined", preset)
	}
	if !hasCompileCommands(buildDir) {
		return "", fmt.Errorf(
			"cmake preset %q succeeded but compile_commands.json was not created in %s",
			preset, buildDir,
		)
	}

	return buildDir, nil
}

func resolvePresetBinaryDirForProject(projectRoot, preset string) (string, error) {
	presetsPath := filepath.Join(projectRoot, "CMakePresets.json")
	data, err := os.ReadFile(presetsPath)
	if err != nil {
		return "", fmt.Errorf("failed to read CMakePresets.json after configure: %w", err)
	}

	var presetsFile cmakePresetsFile
	if err := json.Unmarshal(data, &presetsFile); err != nil {
		return "", fmt.Errorf("failed to parse CMakePresets.json after configure: %w", err)
	}

	byName := make(map[string]cmakeConfigurePreset, len(presetsFile.ConfigurePresets))
	for _, item := range presetsFile.ConfigurePresets {
		byName[item.Name] = item
	}

	return resolvePresetBinaryDir(byName, preset, projectRoot)
}

func generateWithPlainCMake(projectRoot, buildDir string, log logger.Logger) (string, error) {
	if err := os.MkdirAll(buildDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create build directory: %v", err)
	}

	log.Info("Generating compile_commands.json in %s...", buildDir)

	cmd := exec.Command(
		"cmake",
		"-S", projectRoot,
		"-B", buildDir,
		"-DCMAKE_EXPORT_COMPILE_COMMANDS=ON",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(err.Error(), "executable file not found") {
			return "", fmt.Errorf("cmake not found in PATH. Please install CMake to use clangd-query")
		}

		names, namesErr := cmakeConfigurePresetNames(projectRoot)
		if namesErr == nil && len(names) > 0 {
			return "", fmt.Errorf(
				"cmake failed: %v\nOutput: %s\n\nThis project defines CMake presets (%s). "+
					"Build with your usual preset first, or pass --cmake-preset (for example %q).",
				err, output, strings.Join(names, ", "), names[0],
			)
		}

		return "", fmt.Errorf("cmake failed: %v\nOutput: %s", err, output)
	}

	if !hasCompileCommands(buildDir) {
		return "", fmt.Errorf("cmake succeeded but compile_commands.json was not created")
	}

	return buildDir, nil
}

func hasCompileCommands(buildDir string) bool {
	return isCompileCommandsFile(filepath.Join(buildDir, "compile_commands.json"))
}

func isCompileCommandsFile(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}

	var entries []json.RawMessage
	if err := json.Unmarshal(data, &entries); err != nil {
		return false
	}

	return len(entries) > 0
}
