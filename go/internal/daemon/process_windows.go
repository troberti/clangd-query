//go:build windows

package daemon

import (
	"golang.org/x/sys/windows"
)

// Checks whether a process with the given PID is still running. On Windows,
// FindProcess can succeed for a PID that has already exited if a handle is
// still open, so this implementation opens the process with minimal access
// rights and treats STILL_ACTIVE as the liveness check.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	var exitCode uint32
	err = windows.GetExitCodeProcess(handle, &exitCode)
	if err != nil {
		return false
	}

	return exitCode == 259 // STILL_ACTIVE
}
