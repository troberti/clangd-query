package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"clangd-query/internal/logger"
	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches for changes in C++ source files
type FileWatcher struct {
	watcher       *fsnotify.Watcher
	projectRoot   string
	onChange      func([]string)
	debounceTimer *time.Timer
	debounceMu    sync.Mutex
	changedFiles  map[string]bool
	stop          chan struct{}
	logger        logger.Logger
}

// NewFileWatcher creates a new file watcher
func NewFileWatcher(projectRoot string, onChange func([]string), log logger.Logger) (*FileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:      watcher,
		projectRoot:  projectRoot,
		onChange:     onChange,
		changedFiles: make(map[string]bool),
		stop:         make(chan struct{}),
		logger:       log,
	}

	// Add project root and subdirectories
	if err := fw.addDirectoryRecursive(projectRoot); err != nil {
		watcher.Close()
		return nil, err
	}

	// Start watching
	go fw.watch()

	return fw, nil
}

// addDirectoryRecursive adds a directory and all subdirectories to the watcher
func (fw *FileWatcher) addDirectoryRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Ignore errors walking the tree
		}

		// Skip hidden directories and build directories
		if info.IsDir() {
			base := filepath.Base(path)
			if strings.HasPrefix(base, ".") || 
			   base == "build" || 
			   base == "cmake-build-debug" || 
			   base == "cmake-build-release" ||
			   base == "out" ||
			   base == "bin" ||
			   base == "obj" {
				return filepath.SkipDir
			}

			// Add directory to watcher
			if err := fw.watcher.Add(path); err != nil {
				// Ignore errors adding individual directories
				fw.logger.Info("Warning: failed to watch %s: %v", path, err)
			}
		}

		return nil
	})
}

// watch handles file system events
func (fw *FileWatcher) watch() {
	for {
		select {
		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Check if it's a C++ file
			if fw.isCppFile(event.Name) {
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					fw.handleFileChange(event.Name)
				}
			}

			// If a new directory was created, add it to the watcher
			if event.Op&fsnotify.Create != 0 {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					fw.addDirectoryRecursive(event.Name)
				}
			}

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			fw.logger.Error("File watcher error: %v", err)

		case <-fw.stop:
			return
		}
	}
}

// handleFileChange handles a file change event with debouncing
func (fw *FileWatcher) handleFileChange(path string) {
	fw.debounceMu.Lock()
	defer fw.debounceMu.Unlock()

	// Add to changed files
	fw.changedFiles[path] = true

	// Cancel existing timer
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}

	// Start new timer
	fw.debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
		fw.debounceMu.Lock()
		
		// Collect all changed files
		files := make([]string, 0, len(fw.changedFiles))
		for file := range fw.changedFiles {
			files = append(files, file)
		}
		
		// Clear the map
		fw.changedFiles = make(map[string]bool)
		
		fw.debounceMu.Unlock()

		// Call the callback
		if len(files) > 0 {
			fw.onChange(files)
		}
	})
}

// isCppFile checks if a file is a C++ source or header file
func (fw *FileWatcher) isCppFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".cpp", ".cc", ".cxx", ".c++",
	     ".h", ".hpp", ".hxx", ".h++",
	     ".c", ".hh":
		return true
	default:
		return false
	}
}

// Stop stops the file watcher
func (fw *FileWatcher) Stop() error {
	close(fw.stop)
	
	fw.debounceMu.Lock()
	if fw.debounceTimer != nil {
		fw.debounceTimer.Stop()
	}
	fw.debounceMu.Unlock()
	
	return fw.watcher.Close()
}