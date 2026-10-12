//go:build windows

package commands

// netexec_windows.go — configuração de grupo de processos para Windows.
//
// Windows não tem grupos de processos POSIX. Setpgid/SIGKILL de grupo não existem.
// WaitDelay ainda garante que o trackfw retorna dentro de timeout + WaitDelay;
// o neto (ssh) pode sobreviver como órfão (ADR D7/A2, RS-2).
//
// hasControllingTTY é definido aqui como seam testável; retorna false porque o
// mecanismo de ctty não se aplica ao Windows da mesma forma.

import "os/exec"

// hasControllingTTY always returns false on Windows.
// Provided as a testable seam (same signature as the Unix version).
var hasControllingTTY = func() bool { return false }

// applyGroupDeadline is a no-op on Windows.
// Only WaitDelay acts as the safety net (RS-2).
func applyGroupDeadline(_ *exec.Cmd) {}
