//go:build !windows

package commands

import (
	"syscall"
)

// killProcess sends SIGKILL to the process with the given PID.
// Used to clean up helper processes that survived as orphans.
func killProcess(pid int) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// isProcessAlive reports whether a process with the given PID exists and is running.
// On Unix, sending signal 0 checks existence without affecting the process.
func isProcessAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}
