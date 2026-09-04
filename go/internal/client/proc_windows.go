//go:build windows

package client

import (
	"os/exec"
	"syscall"
)

// Configures the command to start in a new process group so the daemon is
// detached from the client's console session.
func configureDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
