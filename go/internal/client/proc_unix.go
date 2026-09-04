//go:build !windows

package client

import (
	"os/exec"
	"syscall"
)

// Configures the command to start in a new process group so the daemon is
// detached from the client's terminal and signal handling.
func configureDetachedProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}
