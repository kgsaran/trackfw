package validator

// validator_guard_binary_probe.go — ML-2B (REQ-2026-09-05)
//
// Infraestrutura partilhada pela nova lógica de validação do subcomando `trackfw guard`:
//   - Tipo guardShellFamily e mapeamento normativo de CLI → família (um sítio só).
//   - Sonda do binário: existe no PATH? Tem o subcomando guard?
//   - Verificação Windows-only: shim .ps1 + política Restricted → falha aberta.
//   - Regra `trackfw_binary_in_project_root`: .exe/.cmd/.bat na raiz.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// guardProbeEndedByDeadline reporta se foi o prazo do contexto que encerrou o comando.
//
// Converge para resolveNetworkErr (internal/commands/netexec.go, ADR D1 do prazo de rede),
// que ja faz esta pergunta ANTES de classificar o erro do exec: `if ctx.Err() != nil`.
// O caminho inverso — classificar primeiro — confunde prazo estourado com veredito do
// processo, porque exec.CommandContext MATA o filho no prazo e cmd.Output()/cmd.Run()
// devolvem *exec.ExitError. Medido em 2026-10-19: com o trackfw PRESENTE no PATH do login
// shell, prazo de 1s/300ms/1ms produzia a violacao de "nao acha o trackfw".
//
// Um sitio so, consumido pelas duas sondas desta regra (guardRunProbe e
// guardRunBashLoginProbe), pelo mesmo motivo escrito no resolveNetworkErr: a
// classificacao e testada uma vez e aplicada de forma consistente.
func guardProbeEndedByDeadline(ctx context.Context) bool {
	return ctx.Err() != nil
}

// guardProbeTimeout e o prazo das sondas desta regra (binario, login shell e politica
// do PowerShell). Vive numa var de pacote pelo mesmo motivo do cmdWaitDelay em
// internal/commands/netexec.go: ser substituivel nos testes que exercitam
// comportamento especifico de prazo. Sem isso, o terceiro caso do doc de
// guardRunBashLoginProbe so seria testavel com um processo que dorme 5 s.
var guardProbeTimeout = 5 * time.Second

// errGuardProbeDeadline marca o erro de guardRunProbe cujo processo foi morto pelo prazo.
//
// A seam guardRunProbe devolve `error` e e substituida em muitos testes, entao mudar a
// assinatura para transportar "foi o prazo" custaria reescrever esses testes. O sentinela
// transporta a informacao sem tocar na forma: o chamador pergunta com errors.Is.
var errGuardProbeDeadline = errors.New("guard probe deadline exceeded")

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

// guardExpectedLine retorna a linha exata de hook D11 revised REQ-2026-10-20 fail-closed (com prefixo
// emulate -R sh) para a família de shell e nome de subcomando dados
// (REQ-2026-10-20 / REQ-2026-10-20 / ADR-2026-10-04 D11 revised). O subcmdName pode incluir
// flags (ex.: "credential --global").
func guardExpectedLine(subcmdName string, fam guardShellFamily) string {
	if fam == guardShellFamilyCmdExe {
		return "trackfw guard " + subcmdName + " || exit 2"
	}
	return `emulate -R sh 2>${null-/dev/null}; $LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard ` + subcmdName + `; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
}

// guardD11RevisedLegacyLine retorna a linha D11 revised pré-REQ-2026-10-20 (com 4º trecho PS mas sem
// prefixo). Esta forma aborta o zsh via EQUALS/NOMATCH expansion; é tratada como
// warning de migração (via validateGuardHookD11RevisedLegacyWarnings), não violation.
// Só relevante para a família PS/POSIX.
func guardD11RevisedLegacyLine(subcmdName string, fam guardShellFamily) string {
	if fam == guardShellFamilyCmdExe {
		// cmd.exe family did not change between D11 revised and REQ-2026-10-20.
		return "trackfw guard " + subcmdName + " || exit 2"
	}
	return `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard ` + subcmdName + `; LASTEXITCODE=$((2*!!$?)); $LASTEXITCODE=2*!!$LASTEXITCODE 2>${null-/dev/null}; exit $LASTEXITCODE`
}

// guardD11LegacyLine retorna a linha D11 pré-ML-6C (sem o 4º trecho de normalização PowerShell)
// para a família de shell e nome de subcomando dados. Usada para distinguir a forma D11 legada
// (warning de migração) de uma violação. Só relevante para a família PS/POSIX.
func guardD11LegacyLine(subcmdName string, fam guardShellFamily) string {
	if fam == guardShellFamilyCmdExe {
		// cmd.exe family did not change between D11 and D11 revised.
		return "trackfw guard " + subcmdName + " || exit 2"
	}
	return `$LASTEXITCODE=2 2>${null-/dev/null}; trackfw guard ` + subcmdName + `; LASTEXITCODE=$((2*!!$?)); exit $LASTEXITCODE`
}

// guardD2LegacyLine retorna a linha D2-revised (pré-ML-6B) para a família de shell e nome de
// subcomando dados. Usada para distinguir a forma legada (warning) de uma violação.
func guardD2LegacyLine(subcmdName string, fam guardShellFamily) string {
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
	ctx, cancel := context.WithTimeout(context.Background(), guardProbeTimeout)
	defer cancel()
	// #nosec G204 — bin is the resolved path from exec.LookPath, not user input
	cmd := exec.CommandContext(ctx, bin, "guard", "--help") //nolint:gosec
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()
	if err != nil && guardProbeEndedByDeadline(ctx) {
		// Prazo estourado nao e "o binario nao tem o subcomando guard": o processo foi
		// morto antes de responder. O sentinela deixa o chamador pular em vez de acusar.
		return errors.Join(errGuardProbeDeadline, err)
	}
	return err
}

// guardFindGitBashExe finds the bash.exe from Git for Windows.
// Resolution order:
//  1. exec.LookPath("bash") — used if the resolved path contains "/git/" (case-insensitive,
//     forward-slash normalized), indicating it is from a Git for Windows installation.
//  2. Known installation paths via %ProgramFiles%, %ProgramFiles(x86)%, %LOCALAPPDATA%.
//
// Returns "" when Git Bash is not found.
// Only called when CurrentGOOS == "windows". Replaced in tests.
var guardFindGitBashExe = func() string {
	if p, err := exec.LookPath("bash"); err == nil {
		if strings.Contains(strings.ToLower(filepath.ToSlash(p)), "/git/") {
			return p
		}
	}
	for _, spec := range []struct{ envKey, rel string }{
		{"ProgramFiles", filepath.Join("Git", "bin", "bash.exe")},
		{"ProgramFiles(x86)", filepath.Join("Git", "bin", "bash.exe")},
		{"LOCALAPPDATA", filepath.Join("Programs", "Git", "bin", "bash.exe")},
	} {
		if base := os.Getenv(spec.envKey); base != "" {
			candidate := filepath.Join(base, spec.rel)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	return ""
}

// guardRunBashLoginProbe runs `<bashExe> -lc 'command -v trackfw'` with a 5 s timeout
// and no inherited stdin or stderr. Returns the trimmed path resolved by the Git Bash
// login shell, plus a boolean indicating whether bash actually ran:
//
//   - ("path", true)  — bash ran (exit 0), path is the last non-empty output line.
//   - ("", true)      — bash ran (exit non-zero), trackfw was not found in login PATH.
//   - ("", false)     — bash could not start, or the context deadline expired
//     (o prazo e discriminado por guardProbeEndedByDeadline ANTES do errors.As).
//
// Login profile noise is handled by taking the last non-empty line of stdout.
// WaitDelay ensures the 5 s context deadline applies even if a grandchild holds the pipe open.
// Only called when CurrentGOOS == "windows". Replaced in tests.
var guardRunBashLoginProbe = func(bashExe string) (resolved string, ran bool) {
	ctx, cancel := context.WithTimeout(context.Background(), guardProbeTimeout)
	defer cancel()
	// #nosec G204 — bashExe comes from guardFindGitBashExe, not user input
	cmd := exec.CommandContext(ctx, bashExe, "-lc", "command -v trackfw") //nolint:gosec
	cmd.Stdin = nil
	cmd.Stderr = nil
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.Output()
	if err != nil {
		if guardProbeEndedByDeadline(ctx) {
			// O prazo estourou: o login shell nao disse nada, e nao se afirma nada.
			// Este era o terceiro caso do doc acima, que o codigo nunca alcancava —
			// exec.CommandContext mata o filho no prazo e cmd.Output() devolve
			// *exec.ExitError, entao o errors.As abaixo casava e o veredito saia
			// como "trackfw ausente do PATH".
			return "", false
		}
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			// Bash couldn't start — skip entirely.
			return "", false
		}
		// Bash ran but command -v exited non-zero: trackfw not found.
		return "", true
	}
	// Take the last non-empty line to skip any profile noise printed to stdout.
	lines := strings.Split(strings.TrimRight(string(out), "\r\n"), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return line, true
		}
	}
	return "", true
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
	ctx, cancel := context.WithTimeout(context.Background(), guardProbeTimeout)
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
		if errors.Is(probeErr, errGuardProbeDeadline) {
			// Prazo estourado: nada afirmado. Melhor nao dizer nada que acusar o binario.
			return nil
		}
		return []string{fmt.Sprintf(
			"the trackfw resolved at %q does not have the guard subcommand — "+
				"the hook will fail open; update trackfw (make install / "+
				"pip install --upgrade trackfw / npm install -g trackfw)",
			bin,
		)}
	}

	// Windows-only checks — only relevant when a PS/POSIX-family entry exists.
	if CurrentGOOS == "windows" && hasPSPosixEntry {
		if msgs := guardCheckPS1RestrictedPolicy(); len(msgs) > 0 {
			return msgs
		}
		// ML-5D: also probe the binary that the Git Bash login shell resolves, because
		// Claude Code runs hook commands via Git Bash whose login profile can prepend ~/bin
		// to PATH, placing an old (no-guard) binary ahead of the one exec.LookPath sees.
		if msgs := guardGitBashBinaryProbe(bin); len(msgs) > 0 {
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
// guardPosixToWindowsPath, guardStripExe — helpers
// --------------------------------------------------------------------------

// guardPosixToWindowsPath converts a Git Bash POSIX-style path like /c/Users/Lab/bin/trackfw
// to a Windows path C:\Users\Lab\bin\trackfw. Returns the input unchanged if it does not
// match the /X/... pattern (single drive letter followed by /), so paths like /usr/bin/...
// or /mingw64/... are passed through as-is (and callers should then skip probing them).
func guardPosixToWindowsPath(posix string) string {
	// Must match /X/... where X is a single ASCII drive letter.
	if len(posix) < 3 || posix[0] != '/' || posix[2] != '/' {
		return posix
	}
	drive := posix[1]
	if !((drive >= 'a' && drive <= 'z') || (drive >= 'A' && drive <= 'Z')) {
		return posix
	}
	rest := strings.ReplaceAll(posix[3:], "/", "\\")
	return strings.ToUpper(string(drive)) + ":\\" + rest
}

// guardStripExe strips a trailing ".exe" (case-insensitive) from path.
// Used to normalise Git Bash output (/c/tools/trackfw → C:\tools\trackfw) against
// exec.LookPath output (C:\tools\trackfw.exe) before comparing.
func guardStripExe(path string) string {
	if strings.HasSuffix(strings.ToLower(path), ".exe") {
		return path[:len(path)-4]
	}
	return path
}

// --------------------------------------------------------------------------
// guardGitBashBinaryProbe — ML-5D (REQ-2026-09-05)
// --------------------------------------------------------------------------

// guardGitBashBinaryProbe checks whether the trackfw binary resolved by the Git Bash
// login shell diverges from windowsResolvedBin (the binary already probed via exec.LookPath)
// and, if so, whether that login-resolved binary has the guard subcommand.
//
// Background (ML-5B, 2026-10-07): Claude Code runs hook commands via Git Bash
// (/usr/bin/bash). The Git Bash login profile prepends ~/bin to PATH, which can place an
// old trackfw binary (without guard) ahead of the one exec.LookPath sees. The main probe
// in guardBinaryProbeOnce reports "OK" while the hook fails open.
//
// Decision rules:
//   - Git Bash not found → skip (nil).
//   - bash ran, trackfw not found in login PATH → violation (hook exits 127, fail-open).
//   - bash ran, path not convertible to a Windows path → skip (MSYS /usr/ or /mingw64/).
//   - same binary as windowsResolvedBin (EqualFold, .exe stripped) → skip (already probed).
//   - different binary, guard --help fails → violation naming the Git Bash-resolved path.
//   - different binary, guard --help passes → nil (no problem).
//
// Only called when CurrentGOOS == "windows" and hasPSPosixEntry is true.
func guardGitBashBinaryProbe(windowsResolvedBin string) []string {
	bashExe := guardFindGitBashExe()
	if bashExe == "" {
		return nil
	}

	loginPath, ran := guardRunBashLoginProbe(bashExe)
	if !ran {
		// Bash could not start or timed out — skip; don't emit a false positive.
		return nil
	}

	if loginPath == "" {
		// Bash ran but trackfw was not found in the Git Bash login PATH.
		return []string{
			"the Git Bash login shell (used by Claude Code on Windows to run hooks) " +
				"cannot find trackfw in its PATH — the hook will fail open (exit 127); " +
				"install trackfw on a path that the Git Bash login profile resolves " +
				"(the login profile prepends ~/bin to PATH — add a current trackfw there or " +
				"ensure the installed binary is on a PATH entry that login inherits)",
		}
	}

	// Normalise the POSIX-style Git Bash path to a Windows path.
	winPath := guardPosixToWindowsPath(loginPath)

	// If the path could not be converted (e.g. /usr/bin/trackfw, /mingw64/bin/trackfw),
	// we cannot reliably probe it as a Windows executable — skip.
	if !strings.Contains(winPath, ":\\") {
		return nil
	}

	// If it's the same binary (ignoring case and the optional .exe extension), it was already
	// probed by guardBinaryProbeOnce — no additional check needed.
	if strings.EqualFold(guardStripExe(winPath), guardStripExe(windowsResolvedBin)) {
		return nil
	}

	// Different binary — probe it for the guard subcommand.
	if probeErr := guardRunProbe(winPath); probeErr != nil {
		if errors.Is(probeErr, errGuardProbeDeadline) {
			// Prazo estourado: nada afirmado. Mesmo tratamento do outro consumidor
			// de guardRunProbe (guardBinaryProbeOnce) — o prazo diz que a sonda não
			// terminou, nunca que o binário não tem o subcomando.
			return nil
		}
		return []string{fmt.Sprintf(
			"the Git Bash login shell (used by Claude Code on Windows to run hooks) resolves %s, "+
				"which does not have the guard subcommand — the hook will fail open; "+
				"remove or update that binary "+
				"(the Git Bash login profile prepends ~/bin to PATH)",
			winPath,
		)}
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
