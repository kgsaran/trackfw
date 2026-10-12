//go:build !windows

package commands

// netexec_unix.go — configuração de grupo de processos para Unix.
//
// Em Unix sem terminal de controle (/dev/tty falha ao abrir):
//   - Setpgid: true coloca o filho num processo-grupo próprio.
//   - cmd.Cancel mata o grupo inteiro com SIGKILL no vencimento do contexto.
//   - Isso garante que o neto (ssh) morre junto, sem órfãos (modo D, Wave 0 seção 2.5).
//
// Com terminal de controle (hasControllingTTY() == true):
//   - NÃO aplica Setpgid: o processo em outro grupo não receberia SIGTTIN ao pedir
//     passphrase, e Ctrl-C deixaria de alcançá-lo (ADR D7/A2, RS-6).
//   - Somente WaitDelay atua; o neto pode sobreviver (RS-6).
//
// Em Windows, applyGroupDeadline é no-op (netexec_windows.go).

import (
	"os"
	"os/exec"
	"syscall"
)

// hasControllingTTY reports whether the current process has a controlling terminal.
// Uses /dev/tty as the discriminator (more precise than os.Stdin.Stat(): /dev/null
// is also a char device but has no ctty). Overridable in tests.
var hasControllingTTY = func() bool {
	// write-containment-allowed: O_RDONLY probe — no file is created or mutated; fails fast and returns false when /dev/tty is absent (CI, Windows).
	tty, err := os.OpenFile("/dev/tty", os.O_RDONLY, 0)
	if err == nil {
		tty.Close()
		return true
	}
	return false
}

// applyGroupDeadline configures cmd to kill the entire process group on context expiry
// when there is no controlling terminal. With a ctty, falls back to WaitDelay only.
func applyGroupDeadline(cmd *exec.Cmd) {
	if hasControllingTTY() {
		// Interactive session: only WaitDelay; Setpgid would block ssh passphrase
		// prompts and break Ctrl-C (ADR D7/A2, RS-6).
		return
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Kill the entire process group so the grandchild (ssh) does not survive
		// as an orphan holding the stderr pipe open (mode D, Wave 0 section 2.5).
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
