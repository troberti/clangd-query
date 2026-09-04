//go:build !windows

package daemon

import (
	"syscall"
)

// Checks whether a process with the given PID is still running on the system.
// This is done by sending signal 0 to the process, which performs a permission
// check without actually sending a signal. Returns false for invalid PIDs or
// if the process no longer exists.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Send signal 0 to check if process exists
	err := syscall.Kill(pid, 0)
	return err == nil
}
