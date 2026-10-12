//go:build windows

package commands

import (
	"bytes"
	"os/exec"
	"strconv"
)

// killProcess attempts to terminate the process with the given PID on Windows.
func killProcess(pid int) {
	exec.Command("taskkill", "/F", "/PID", strconv.Itoa(pid)).Run() //nolint:errcheck
}

// isProcessAlive reports whether a process with the given PID exists on Windows.
func isProcessAlive(pid int) bool {
	out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/NH").Output()
	if err != nil {
		return false
	}
	return bytes.Contains(out, []byte(strconv.Itoa(pid)))
}
