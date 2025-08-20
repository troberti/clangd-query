package daemon

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// LockInfo contains metadata about a running daemon
type LockInfo struct {
	PID           int    `json:"pid"`
	SocketPath    string `json:"socketPath"`
	BuildTime     int64  `json:"buildTime"`
	ProjectRoot   string `json:"projectRoot"`
}

// GetSocketPath returns the socket path for a project
func GetSocketPath(projectRoot string) string {
	hash := md5.Sum([]byte(projectRoot))
	socketName := fmt.Sprintf("clangd-query-go-%x.sock", hash)
	return filepath.Join(os.TempDir(), socketName)
}

// GetLockPath returns the lock file path for a project
func GetLockPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".clangd-query-go.lock")
}

// GetLogPath returns the log file path for a project
func GetLogPath(projectRoot string) string {
	cacheDir := filepath.Join(projectRoot, ".cache", "clangd-query-go")
	os.MkdirAll(cacheDir, 0755)
	return filepath.Join(cacheDir, "daemon.log")
}

// WriteLockFile writes daemon metadata to the lock file
func WriteLockFile(projectRoot string, pid int, socketPath string) error {
	lockPath := GetLockPath(projectRoot)
	
	// Get build time of current executable
	execPath, err := os.Executable()
	if err != nil {
		return err
	}
	
	stat, err := os.Stat(execPath)
	if err != nil {
		return err
	}
	
	lockInfo := LockInfo{
		PID:         pid,
		SocketPath:  socketPath,
		BuildTime:   stat.ModTime().Unix(),
		ProjectRoot: projectRoot,
	}
	
	data, err := json.MarshalIndent(lockInfo, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(lockPath, data, 0644)
}

// ReadLockFile reads daemon metadata from the lock file
func ReadLockFile(projectRoot string) (*LockInfo, error) {
	lockPath := GetLockPath(projectRoot)
	
	data, err := os.ReadFile(lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No lock file
		}
		return nil, err
	}
	
	var lockInfo LockInfo
	if err := json.Unmarshal(data, &lockInfo); err != nil {
		return nil, err
	}
	
	return &lockInfo, nil
}

// RemoveLockFile removes the lock file
func RemoveLockFile(projectRoot string) error {
	lockPath := GetLockPath(projectRoot)
	err := os.Remove(lockPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// IsProcessAlive checks if a process with the given PID is still running
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	
	// Send signal 0 to check if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}

// IsDaemonStale checks if the daemon needs to be restarted
func IsDaemonStale(lockInfo *LockInfo) bool {
	// Check if process is alive
	if !IsProcessAlive(lockInfo.PID) {
		return true
	}
	
	// Check if binary has been updated
	execPath, err := os.Executable()
	if err != nil {
		return false // Can't determine, assume not stale
	}
	
	stat, err := os.Stat(execPath)
	if err != nil {
		return false // Can't determine, assume not stale
	}
	
	// If binary is newer than lock file, daemon is stale
	return stat.ModTime().Unix() > lockInfo.BuildTime
}

// CleanupSocket removes the socket file if it exists
func CleanupSocket(socketPath string) error {
	err := os.Remove(socketPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetBuildTime returns the modification time of the current executable
func GetBuildTime() (int64, error) {
	execPath, err := os.Executable()
	if err != nil {
		return 0, err
	}
	
	stat, err := os.Stat(execPath)
	if err != nil {
		return 0, err
	}
	
	return stat.ModTime().Unix(), nil
}

// TruncateLogFile truncates the log file if it's too large
func TruncateLogFile(projectRoot string, maxSize int64) error {
	logPath := GetLogPath(projectRoot)
	
	stat, err := os.Stat(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No log file yet
		}
		return err
	}
	
	if stat.Size() > maxSize {
		// Keep last 10% of the file
		keepSize := maxSize / 10
		
		file, err := os.OpenFile(logPath, os.O_RDWR, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		
		// Seek to position where we want to start keeping
		_, err = file.Seek(stat.Size()-keepSize, 0)
		if err != nil {
			return err
		}
		
		// Read the remaining content
		remaining := make([]byte, keepSize)
		n, err := file.Read(remaining)
		if err != nil && err != io.EOF {
			return err
		}
		
		// Write header and remaining content to new file
		tempFile := logPath + ".tmp"
		header := fmt.Sprintf("=== Log truncated at %s ===\n", time.Now().Format(time.RFC3339))
		content := append([]byte(header), remaining[:n]...)
		
		if err := os.WriteFile(tempFile, content, 0644); err != nil {
			return err
		}
		
		// Replace old file with new
		return os.Rename(tempFile, logPath)
	}
	
	return nil
}