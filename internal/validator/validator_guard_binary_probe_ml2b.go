package validator

// validator_guard_binary_probe_ml2b.go — ML-2B (REQ-2026-09-05)
//
// Infraestrutura partilhada pela nova lógica de validação do subcomando `trackfw guard`:
//   - Tipo guardShellFamily e mapeamento normativo de CLI → família (um sítio só).
//   - Sonda do binário: existe no PATH? Tem o subcomando guard?
//   - Verificação Windows-only: shim .ps1 + política Restricted → falha aberta.
//   - Regra `trackfw_binary_in_project_root`: .exe/.cmd/.bat na raiz.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// guardShellFamily classifica o ambiente de shell que executa o hook, determinando a forma
// exata da linha emitida para cada CLI (ADR-2026-10-04, D2 revista).
//
// Tabela normativa — um sítio só (ML-2B):
//
//	guardShellFamilyPSPosix — PowerShell ou POSIX sh/bash
//	  Linha emitida: "trackfw guard <nome>; exit $LASTEXITCODE"
//	  CLIs: Claude Code, Codex, Gemini, Cursor, Copilot, Windsurf
//
//	guardShellFamilyCmdExe  — cmd.exe
//	  Linha emitida: "trackfw guard <nome>" (sem sufixo)
//	  CLIs: Kiro, Amazon Q
//
// ML-2C: Windsurf e Amazon Q foram adicionados a credentialGuardHookFiles (validator_credential_guard.go).
// globalGuardConfigFiles não os inclui: o gerador não emite harness global para nenhum dos dois.
type guardShellFamily int

const (
	// guardShellFamilyPSPosix: hook executado por PowerShell ou sh/bash.
	guardShellFamilyPSPosix guardShellFamily = iota
	// guardShellFamilyCmdExe: hook executado por cmd.exe.
	guardShellFamilyCmdExe
)

// guardExpectedLine retorna a linha exata de hook para a família de shell e nome de subcomando
// dados. O subcmdName pode incluir flags (ex.: "credential --global").
func guardExpectedLine(subcmdName string, fam guardShellFamily) string {
	if fam == guardShellFamilyCmdExe {
		return "trackfw guard " + subcmdName
	}
	return "trackfw guard " + subcmdName + "; exit $LASTEXITCODE"
}

// --------------------------------------------------------------------------
// Sondas — seams substituíveis em testes
// --------------------------------------------------------------------------

// guardLookupBinary resolves the `trackfw` binary in PATH. Replaced in tests.
var guardLookupBinary = func(name string) (string, error) {
	return exec.LookPath(name)
}

// guardRunProbe runs `<bin> guard --help` and returns nil on exit 0.
// Timeout: 5 s; stdin/stdout/stderr not inherited (only the exit code matters, and
// nil pipes prevent Wait from hanging on a grandchild holding the pipe open).
// Replaced in tests.
var guardRunProbe = func(bin string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// #nosec G204 — bin is the resolved path from exec.LookPath, not user input
	cmd := exec.CommandContext(ctx, bin, "guard", "--help") //nolint:gosec
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// guardFindTrackfwForPS walks PATH the way PowerShell resolves a command: for each directory
// in order, it checks trackfw.ps1 first (ExternalScript wins over Application within the
// same dir — measured portabilidade doc §D5), then .exe, .cmd, .bat. Returns the first
// match, or "" if not found.
// Only called when CurrentGOOS == "windows". Replaced in tests.
var guardFindTrackfwForPS = func() string {
	dirs := filepath.SplitList(os.Getenv("PATH"))
	for _, dir := range dirs {
		for _, ext := range []string{".ps1", ".exe", ".cmd", ".bat"} {
			candidate := filepath.Join(dir, "trackfw"+ext)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

// guardGetPSPolicy queries the effective PowerShell execution policy and returns the
// policy string (e.g. "Restricted", "RemoteSigned") or "" on error.
//
// PSExecutionPolicyPreference is unset from the child environment before the query:
// a parent PowerShell with -ExecutionPolicy Bypass exports that variable, which child
// processes inherit, masking the real Restricted policy (measured portabilidade doc
// lines 245-248 / ML-1D measurement).
// Only called when CurrentGOOS == "windows". Replaced in tests.
var guardGetPSPolicy = func() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-Command", "Get-ExecutionPolicy") //nolint:gosec
	cmd.Stdin = nil
	// Strip PSExecutionPolicyPreference from the child env.
	env := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "PSExecutionPolicyPreference=") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// --------------------------------------------------------------------------
// guardBinaryProbeOnce — probe centralizado
// --------------------------------------------------------------------------

// guardBinaryProbeOnce runs the binary probe and returns zero or more violation messages.
// Called at most once per validateGuardHookResolvable invocation when a hook file references
// the new `trackfw guard …` subcommand form.
//
//   - `trackfw` absent from PATH → violation.
//   - `trackfw guard --help` exits non-zero → violation (subcomando guard ausente).
//   - Windows + hasPSPosixEntry: shim .ps1 + política Restricted → violation
//     (ML-1D: com o sufixo `; exit $LASTEXITCODE` a PSSecurityException sai 0 = falha aberta).
func guardBinaryProbeOnce(hasPSPosixEntry bool) []string {
	bin, err := guardLookupBinary("trackfw")
	if err != nil {
		return []string{
			"trackfw binary not found in PATH — the hook will fail open; " +
				"install trackfw (make install / pip install -g trackfw / npm install -g trackfw) " +
				"or ensure it is on PATH",
		}
	}

	if probeErr := guardRunProbe(bin); probeErr != nil {
		return []string{fmt.Sprintf(
			"the trackfw resolved at %q does not have the guard subcommand — "+
				"the hook will fail open; update trackfw (make install / "+
				"pip install --upgrade trackfw / npm install -g trackfw)",
			bin,
		)}
	}

	// Windows-only PS1 check — only relevant when a PS/POSIX-family entry exists.
	if CurrentGOOS == "windows" && hasPSPosixEntry {
		if msgs := guardCheckPS1RestrictedPolicy(); len(msgs) > 0 {
			return msgs
		}
	}
	return nil
}

// guardCheckPS1RestrictedPolicy detects the npm .ps1 shim + Restricted policy scenario.
// Portabilidade doc (§D5 risk / ML-1D): under Restricted, the PSSecurityException from loading
// .ps1 exits 0 (not 1) when the hook has `; exit $LASTEXITCODE` — fail-open for ALL PS/POSIX CLIs.
func guardCheckPS1RestrictedPolicy() []string {
	resolved := guardFindTrackfwForPS()
	if resolved == "" || !strings.HasSuffix(strings.ToLower(resolved), ".ps1") {
		return nil
	}
	policy := guardGetPSPolicy()
	if strings.EqualFold(policy, "Restricted") {
		return []string{
			"trackfw resolves to a PowerShell script (.ps1) under Restricted execution policy — " +
				"the hook will fail open (exit 0 instead of exit 2); run: " +
				"Set-ExecutionPolicy -Scope CurrentUser RemoteSigned",
		}
	}
	return nil
}

// --------------------------------------------------------------------------
// validateTrackfwBinaryInProjectRoot — regra trackfw_binary_in_project_root
// --------------------------------------------------------------------------

// validateTrackfwBinaryInProjectRoot detects trackfw.exe / trackfw.cmd / trackfw.bat in the
// project root (ADR-2026-10-04 risk note). cmd.exe searches CWD before PATH, so Kiro and
// Amazon Q would run the local binary instead of the installed one.
//
// Case-insensitive comparison (NTFS is case-insensitive; os.ReadDir returns the on-disk case).
func validateTrackfwBinaryInProjectRoot() ([]string, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	targets := map[string]struct{}{
		"trackfw.exe": {},
		"trackfw.cmd": {},
		"trackfw.bat": {},
	}

	var msgs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if _, found := targets[strings.ToLower(e.Name())]; found {
			msgs = append(msgs, fmt.Sprintf(
				"%s exists in the project root — cmd.exe searches the current directory before PATH, "+
					"so Kiro and Amazon Q will run this local binary instead of the installed trackfw; "+
					"remove it from the project root",
				e.Name(),
			))
		}
	}
	return msgs, nil
}
