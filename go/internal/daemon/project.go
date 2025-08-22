package daemon

import (
	"fmt"
	"os"
	"path/filepath"
)

// FindProjectRoot finds the project root by looking for CMakeLists.txt
func FindProjectRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		cmakePath := filepath.Join(dir, "CMakeLists.txt")
		if _, err := os.Stat(cmakePath); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root of filesystem
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no CMakeLists.txt found in any parent directory of %s", startDir)
}
