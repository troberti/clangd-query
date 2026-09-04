package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Holds the subset of CMakePresets.json fields needed to locate build
// directories and run preset-based configuration.
type cmakePresetsFile struct {
	ConfigurePresets []cmakeConfigurePreset `json:"configurePresets"`
}

type cmakeConfigurePreset struct {
	Name      string      `json:"name"`
	Inherits  interface{} `json:"inherits"`
	BinaryDir string      `json:"binaryDir"`
}

// Collects candidate build directories declared in CMakePresets.json.
// Inheritance between presets is resolved so derived presets inherit a
// parent's binaryDir when they do not define their own.
func cmakePresetBuildDirs(projectRoot string) ([]string, error) {
	presetsPath := filepath.Join(projectRoot, "CMakePresets.json")
	data, err := os.ReadFile(presetsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var presetsFile cmakePresetsFile
	if err := json.Unmarshal(data, &presetsFile); err != nil {
		return nil, fmt.Errorf("failed to parse CMakePresets.json: %w", err)
	}

	byName := make(map[string]cmakeConfigurePreset, len(presetsFile.ConfigurePresets))
	for _, preset := range presetsFile.ConfigurePresets {
		byName[preset.Name] = preset
	}

	seen := make(map[string]bool)
	var dirs []string
	for _, preset := range presetsFile.ConfigurePresets {
		binaryDir, err := resolvePresetBinaryDir(byName, preset.Name, projectRoot)
		if err != nil {
			continue
		}
		if binaryDir == "" || seen[binaryDir] {
			continue
		}
		seen[binaryDir] = true
		dirs = append(dirs, binaryDir)
	}

	return dirs, nil
}

// Returns configure preset names declared in CMakePresets.json.
func cmakeConfigurePresetNames(projectRoot string) ([]string, error) {
	presetsPath := filepath.Join(projectRoot, "CMakePresets.json")
	data, err := os.ReadFile(presetsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var presetsFile cmakePresetsFile
	if err := json.Unmarshal(data, &presetsFile); err != nil {
		return nil, fmt.Errorf("failed to parse CMakePresets.json: %w", err)
	}

	names := make([]string, 0, len(presetsFile.ConfigurePresets))
	for _, preset := range presetsFile.ConfigurePresets {
		if preset.Name != "" {
			names = append(names, preset.Name)
		}
	}
	return names, nil
}

func resolvePresetBinaryDir(byName map[string]cmakeConfigurePreset, presetName, projectRoot string) (string, error) {
	visited := make(map[string]bool)
	return resolvePresetBinaryDirRecursive(byName, presetName, projectRoot, visited)
}

func resolvePresetBinaryDirRecursive(
	byName map[string]cmakeConfigurePreset,
	presetName, projectRoot string,
	visited map[string]bool,
) (string, error) {
	if visited[presetName] {
		return "", fmt.Errorf("cyclic preset inheritance involving %q", presetName)
	}
	visited[presetName] = true

	preset, ok := byName[presetName]
	if !ok {
		return "", fmt.Errorf("unknown preset %q", presetName)
	}

	binaryDir := preset.BinaryDir
	if binaryDir == "" {
		for _, parentName := range inheritNames(preset.Inherits) {
			parentDir, err := resolvePresetBinaryDirRecursive(byName, parentName, projectRoot, visited)
			if err != nil {
				continue
			}
			if parentDir != "" {
				binaryDir = parentDir
				break
			}
		}
	}

	if binaryDir == "" {
		return "", nil
	}

	return expandCMakeVariables(binaryDir, projectRoot, presetName), nil
}

func inheritNames(inherits interface{}) []string {
	switch value := inherits.(type) {
	case string:
		if value != "" {
			return []string{value}
		}
	case []interface{}:
		names := make([]string, 0, len(value))
		for _, item := range value {
			if name, ok := item.(string); ok && name != "" {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

func expandCMakeVariables(value, projectRoot, presetName string) string {
	sourceParentDir := filepath.Dir(projectRoot)
	replacer := strings.NewReplacer(
		"${sourceDir}", projectRoot,
		"$env{sourceDir}", projectRoot,
		"${fileDir}", projectRoot,
		"$env{fileDir}", projectRoot,
		"${sourceParentDir}", sourceParentDir,
		"$env{sourceParentDir}", sourceParentDir,
		"${presetName}", presetName,
		"$env{presetName}", presetName,
	)
	return filepath.Clean(filepath.FromSlash(replacer.Replace(value)))
}
