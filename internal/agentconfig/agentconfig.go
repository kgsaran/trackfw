// Package agentconfig resolves the home directory (and config directory) for
// each agent CLI supported by trackfw. It is the single source of truth for
// "which directory does this CLI read from?" — used by both the update harness
// (internal/generators/update.go) and the validator
// (internal/validator/validator_git_branch_guard.go,
// internal/validator/validator_credential_guard.go).
//
// Motivation (ADR-2026-10-14, REQ-2026-09-01, ML-1A): on Windows, HOME ≠
// USERPROFILE when the user runs trackfw from Git Bash. The harness was writing
// the guard to HOME/.claude/settings.json while Claude Code reads
// USERPROFILE/.claude/settings.json — a "ghost installation" that passes
// validate but never reaches the agent (measured 2026-10-14,
// docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md).
//
// Design (A5 from the Wave 0 approval):
//
//   - CLIHomeResolver is the injectable interface.
//   - OSCLIHomeResolver is the production implementation; it reads GOOS and
//     os.UserHomeDir from pluggable platform inputs, which SetPlatformForTest
//     overrides to exercise the Windows branch on macOS/Linux without touching
//     the real home directory.
//   - DefaultResolver is the package-level resolver used by all production
//     callers; tests may also replace it with FixedCLIHomeResolver.
//   - ClaudeConfigDir returns the directory containing Claude's settings.json,
//     honouring CLAUDE_CONFIG_DIR when set (A1).
package agentconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kgsaran/trackfw/internal/fileutil"
	"github.com/kgsaran/trackfw/internal/homedir"
)

// CLI name constants used by HomeFor and ClaudeConfigDir.
const (
	CLIClaude       = "claude"
	CLICodex        = "codex"
	CLIGemini       = "gemini"
	CLIAntigravity  = "antigravity"
	CLICursor       = "cursor"
	CLICopilot      = "copilot"
	CLIWindsurf     = "windsurf"
	CLIKiro         = "kiro"
)

// CLIHomeResolver resolves the home directory for a given agent CLI.
// On macOS/Linux, HomeFor typically returns homedir.Dir() (HOME first).
// On Windows, Claude Code reads USERPROFILE (os.UserHomeDir()), not $HOME —
// measured 2026-10-14 (docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md).
//
// D3 of ADR-2026-10-14: the writer (update harness) and the auditor (validate)
// MUST call the SAME function for each CLI so they agree on the target path.
type CLIHomeResolver interface {
	HomeFor(cli string) (string, error)
}

// testGOOS overrides runtime.GOOS in tests. Empty = use runtime.GOOS.
// Not goroutine-safe.
var testGOOS string

// testUserHomeDir overrides os.UserHomeDir in tests. Nil = use os.UserHomeDir.
// Not goroutine-safe.
var testUserHomeDir func() (string, error)

// SetPlatformForTest overrides the platform inputs used by OSCLIHomeResolver.
// Returns a restore function that reverts the override.
//
// Not goroutine-safe: do not replace concurrently with HomeFor calls.
//
// Example — simulate Windows with USERPROFILE pointing at dir:
//
//	restore := agentconfig.SetPlatformForTest("windows", func() (string, error) {
//	    return dir, nil
//	})
//	defer restore()
func SetPlatformForTest(goos string, userHomeDir func() (string, error)) func() {
	oldGOOS, oldFn := testGOOS, testUserHomeDir
	testGOOS, testUserHomeDir = goos, userHomeDir
	return func() { testGOOS, testUserHomeDir = oldGOOS, oldFn }
}

func effectiveGOOS() string {
	if testGOOS != "" {
		return testGOOS
	}
	return runtime.GOOS
}

func effectiveUserHomeDir() (string, error) {
	if testUserHomeDir != nil {
		return testUserHomeDir()
	}
	return os.UserHomeDir()
}

// OSCLIHomeResolver is the production CLIHomeResolver.
//
// Claude Code home resolution (ADR-2026-10-14 D1):
//   - On Windows: os.UserHomeDir() — reads USERPROFILE env var (confirmed in
//     $(go env GOROOT)/src/os/file.go; no fallback to Shell API).
//   - On other platforms: homedir.Dir() — HOME first, os.UserHomeDir() fallback
//     (same as today, unaffected by this ML on macOS/Linux).
//
// All other CLIs — behaviour unchanged, inferidos per D2 of ADR-2026-10-14:
//   - homedir.Dir() on all platforms.
//
// Measurement status (Wave 0, 2026-10-14):
//
//   - Claude Code:           MEASURED  (Passo 3a, Windows ARM64, três terminais)
//   - Gemini CLI:            INFERIDO  (proxy Node.js medido)
//   - Codex CLI:             INFERIDO  (fonte secundária Rust)
//   - Cursor:                INFERIDO  (forum)
//   - GitHub Copilot CLI:    INFERIDO  (doc oficial cita $HOME sem precisar mecanismo Windows)
//   - Windsurf:              INFERIDO  — resíduo declarado A8 (três sítios divergentes; medir antes de decidir)
//   - Kiro:                  INFERIDO  (doc oficial)
//
// References:
//
//	docs/seguranca/2026-10-14-wave0-home-por-cli-no-windows.md (Seção 1)
//	docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md
type OSCLIHomeResolver struct{}

// HomeFor returns the home directory for the given CLI.
// See OSCLIHomeResolver documentation for per-CLI resolution rules.
func (OSCLIHomeResolver) HomeFor(cli string) (string, error) {
	if effectiveGOOS() == "windows" && (cli == CLIClaude || cli == CLICodex) {
		// Codex 0.169.0 on Windows resolves the home directory via the Windows
		// profile API (SHGetKnownFolderPath/FOLDERID_Profile), which is what
		// os.UserHomeDir() returns. This is the same mechanism as Claude Code —
		// both ignore HOME and USERPROFILE env vars in favour of the OS API.
		// Measured 2026-11-17: braços (a)/(b)/(d) in
		// docs/portabilidade/2026-11-17-ml-4a-codex-flags-e-home-windows.md §HOME
		// show that neither HOME nor USERPROFILE override the Codex profile path;
		// os.UserHomeDir() matches the API result. Same trade-off accepted for
		// Claude in ADR-2026-10-14 D1 now extended to Codex (D2).
		return effectiveUserHomeDir()
	}
	if effectiveGOOS() == "windows" && cli == CLIAntigravity {
		// Google Antigravity (agy) on Windows reads the global hooks.json from
		// USERPROFILE (os.UserHomeDir() in Go), NOT FOLDERID_Profile — opposite
		// of Codex. Measured 2026-11-18: §12 A3-Win of
		// docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md shows
		// that setting USERPROFILE=C:\agyprobe\altprofile makes agy read the
		// hooks.json at that path; os.UserHomeDir() returns USERPROFILE on Windows.
		return effectiveUserHomeDir()
	}
	return homedir.Dir()
}

// FixedCLIHomeResolver returns the same Home for every CLI. Used in tests to
// simulate platform-specific behaviour without touching real home directories.
type FixedCLIHomeResolver struct {
	Home string
}

// HomeFor returns r.Home regardless of cli.
func (r FixedCLIHomeResolver) HomeFor(_ string) (string, error) {
	return r.Home, nil
}

// DefaultResolver is the CLIHomeResolver used by all production callers.
// Tests may replace it with FixedCLIHomeResolver, or use SetPlatformForTest
// to control the platform inputs of the default OSCLIHomeResolver.
// Not goroutine-safe: do not replace concurrently with HomeFor calls.
var DefaultResolver CLIHomeResolver = OSCLIHomeResolver{}

// normHomePath normalizes a filesystem path for platform-independent comparison.
// Replaces `\` with `/` (Windows path separators) and removes trailing slashes,
// so "C:\Users\Lab" and "c:/users/lab/" both normalize to "c:/users/lab" for
// case-insensitive comparison via strings.EqualFold.
// Used by HomeDivergencePaths (A3, ADR-2026-10-14) and InertClaudeGuardPath (D5).
func normHomePath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	for len(p) > 1 && strings.HasSuffix(p, "/") {
		p = p[:len(p)-1]
	}
	return p
}

// HomeDivergencePaths returns (homeEnv, osUserHome, true) when HOME and
// os.UserHomeDir() differ on Windows (D4, ADR-2026-10-14 A3).
//
// Returns ("", "", false) in all other cases:
//   - outside Windows (real GOOS or via SetPlatformForTest);
//   - when HOME is empty (Git Bash without HOME set);
//   - when effectiveUserHomeDir() returns an error (USERPROFILE absent);
//   - when the two paths are equal after normHomePath + strings.EqualFold —
//     MSYS2 converts /c/Users/Lab to C:\Users\Lab before spawning the Go
//     process (Seção 3.5, Wave 0 2026-10-14), so the default Git Bash case
//     (HOME=/c/Users/Lab, USERPROFILE=C:\Users\Lab) silently passes.
//
// The returned paths are for display only; callers must not resolve, write to,
// or delete anything based on them.
func HomeDivergencePaths() (homeEnv, osUserHome string, divergent bool) {
	if effectiveGOOS() != "windows" {
		return
	}
	homeEnv = os.Getenv("HOME")
	if homeEnv == "" {
		return "", "", false
	}
	var err error
	osUserHome, err = effectiveUserHomeDir()
	if err != nil {
		return "", "", false
	}
	if strings.EqualFold(normHomePath(homeEnv), normHomePath(osUserHome)) {
		return "", "", false
	}
	return homeEnv, osUserHome, true
}

// InertClaudeGuardPath returns (path, true) when HOME/.claude/settings.json
// contains trackfw guard wiring but claudeDir points elsewhere — indicating a
// D5 inert remainder (ADR-2026-10-14 D5).
//
// Returns ("", false) when:
//   - claudeDir equals HOME/.claude (same location, no divergence);
//   - HOME/.claude/settings.json does not exist or cannot be read;
//   - the file does not contain any trackfw guard wiring.
//
// The caller must NOT delete or modify the returned file — D5 is report-only.
func InertClaudeGuardPath(claudeDir string) (path string, present bool) {
	home, err := homedir.Dir()
	if err != nil {
		return "", false
	}
	oldDir := filepath.Join(home, ".claude")
	if strings.EqualFold(normHomePath(claudeDir), normHomePath(oldDir)) {
		return "", false
	}
	p := filepath.Join(oldDir, "settings.json")
	data, err := fileutil.ReadRegularFile(p)
	if err != nil {
		return "", false
	}
	if containsTrackfwGuardMarker(data) {
		return p, true
	}
	return "", false
}

// containsTrackfwGuardMarker reports whether data contains any form of
// trackfw guard wiring. The check covers:
//   - current inline form: "trackfw guard" (post-ML-2A);
//   - legacy .sh forms: "trackfw-credential-guard.sh", "trackfw-git-branch-guard.sh"
//     (pre-ML-2A, may be present in a D5 remainder written by an older version).
func containsTrackfwGuardMarker(data []byte) bool {
	s := string(data)
	return strings.Contains(s, "trackfw guard") ||
		strings.Contains(s, "trackfw-credential-guard.sh") ||
		strings.Contains(s, "trackfw-git-branch-guard.sh")
}

// Source constants returned by ClaudeConfigDirWithSource.
//
//   - ClaudeConfigDirSourceEnvVar: resolution came from the CLAUDE_CONFIG_DIR
//     environment variable (set and non-empty after TrimSpace).
//   - ClaudeConfigDirSourceOSHome: resolution fell through to the OS home
//     (os.UserHomeDir / USERPROFILE) on Windows.
//   - ClaudeConfigDirSourceHome: resolution fell through to homedir.Dir()
//     (HOME env var first) on non-Windows platforms.
const (
	ClaudeConfigDirSourceEnvVar = "CLAUDE_CONFIG_DIR"
	ClaudeConfigDirSourceOSHome = "os-home"
	ClaudeConfigDirSourceHome   = "home"
)

// ClaudeConfigDirWithSource returns the directory that contains Claude Code's
// settings.json together with a string identifying the source of the resolution
// (one of the ClaudeConfigDirSource* constants).
//
// ClaudeConfigDir is a thin wrapper over this function for callers that do not
// need the source.
//
// Resolution order (A1, ADR-2026-10-14 D6):
//
//  1. If CLAUDE_CONFIG_DIR is set and non-empty after TrimSpace:
//     - R-1 (ML-2D): if the value is not absolute after filepath.Clean, returns a
//       named error — converts path-traversal ("../evil", "rel") to an explicit
//       A7-failure rather than silent mis-writes. Spaces-only is treated as
//       "not set" and falls through to step 2.
//     - Otherwise: returns filepath.Clean(CLAUDE_CONFIG_DIR) and source
//       ClaudeConfigDirSourceEnvVar. settings.json lives at the ROOT of this
//       directory (no .claude/ subdir) — measured 2026-10-14 on Windows ARM64
//       (docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md Passo 3.1).
//       Cross-platform.
//
//  2. Otherwise: filepath.Join(r.HomeFor(CLIClaude), ".claude").
//     On Windows (via OSCLIHomeResolver) this uses USERPROFILE (source:
//     ClaudeConfigDirSourceOSHome); on other platforms, HOME (source:
//     ClaudeConfigDirSourceHome).
func ClaudeConfigDirWithSource(r CLIHomeResolver) (dir, source string, err error) {
	raw := os.Getenv("CLAUDE_CONFIG_DIR")
	if strings.TrimSpace(raw) != "" {
		cleaned := filepath.Clean(raw)
		if !filepath.IsAbs(cleaned) {
			return "", "", fmt.Errorf("CLAUDE_CONFIG_DIR %q is not an absolute path", raw)
		}
		return cleaned, ClaudeConfigDirSourceEnvVar, nil
	}
	home, err := r.HomeFor(CLIClaude)
	if err != nil {
		return "", "", err
	}
	dir = filepath.Join(home, ".claude")
	if effectiveGOOS() == "windows" {
		return dir, ClaudeConfigDirSourceOSHome, nil
	}
	return dir, ClaudeConfigDirSourceHome, nil
}

// ClaudeConfigDir returns the directory that contains Claude Code's settings.json.
// It is a wrapper over ClaudeConfigDirWithSource for callers that do not need
// the resolution source.
//
// Callers join "settings.json" to the result:
//
//	dir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
//	if err != nil { /* emit finding per A7 */ }
//	path := filepath.Join(dir, "settings.json")
func ClaudeConfigDir(r CLIHomeResolver) (string, error) {
	dir, _, err := ClaudeConfigDirWithSource(r)
	return dir, err
}

// Source constants returned by CodexConfigDirWithSource.
//
//   - CodexConfigDirSourceEnvVar: resolution came from the CODEX_HOME environment
//     variable (set and non-empty after TrimSpace).
//   - CodexConfigDirSourceOSHome: resolution fell through to the OS home
//     (os.UserHomeDir / Windows profile API) on Windows.
//   - CodexConfigDirSourceHome: resolution fell through to homedir.Dir()
//     (HOME env var first) on non-Windows platforms.
const (
	CodexConfigDirSourceEnvVar = "CODEX_HOME"
	CodexConfigDirSourceOSHome = "os-home"
	CodexConfigDirSourceHome   = "home"
)

// CodexConfigDirWithSource returns the directory that contains Codex CLI's
// hooks.json together with a string identifying the source of the resolution
// (one of the CodexConfigDirSource* constants).
//
// CodexConfigDir is a thin wrapper over this function for callers that do not
// need the source.
//
// Resolution order (ML-4A4, ADR-2026-10-14 D2, measured 2026-11-17 in
// docs/portabilidade/2026-11-17-ml-4a-codex-flags-e-home-windows.md §HOME):
//
//  1. If CODEX_HOME is set and non-empty after TrimSpace:
//     - R-1: if the value is not absolute after filepath.Clean, returns a named
//       error — same A7-failure as CLAUDE_CONFIG_DIR (ML-2D). Spaces-only is
//       treated as "not set" and falls through to step 2.
//     - Otherwise: returns filepath.Clean(CODEX_HOME) and source
//       CodexConfigDirSourceEnvVar. hooks.json lives at the ROOT of this
//       directory (no .codex/ subdir) — measured: braço (c) in the evidence doc
//       above shows homeC\config.toml at the root of CODEX_HOME, and
//       codex-rs/utils/home-dir/src/lib.rs::find_codex_home() confirms the
//       mechanism. Cross-platform.
//
//  2. Otherwise: filepath.Join(r.HomeFor(CLICodex), ".codex").
//     On Windows (via OSCLIHomeResolver) this uses the OS profile API
//     (source: CodexConfigDirSourceOSHome); on other platforms, HOME
//     (source: CodexConfigDirSourceHome).
func CodexConfigDirWithSource(r CLIHomeResolver) (dir, source string, err error) {
	raw := os.Getenv("CODEX_HOME")
	if strings.TrimSpace(raw) != "" {
		cleaned := filepath.Clean(raw)
		if !filepath.IsAbs(cleaned) {
			return "", "", fmt.Errorf("CODEX_HOME %q is not an absolute path", raw)
		}
		return cleaned, CodexConfigDirSourceEnvVar, nil
	}
	home, err := r.HomeFor(CLICodex)
	if err != nil {
		return "", "", err
	}
	dir = filepath.Join(home, ".codex")
	if effectiveGOOS() == "windows" {
		return dir, CodexConfigDirSourceOSHome, nil
	}
	return dir, CodexConfigDirSourceHome, nil
}

// CodexConfigDir returns the directory that contains Codex CLI's hooks.json.
// It is a wrapper over CodexConfigDirWithSource for callers that do not need
// the resolution source.
//
// Callers join "hooks.json" to the result:
//
//	dir, err := agentconfig.CodexConfigDir(agentconfig.DefaultResolver)
//	if err != nil { /* emit finding per A7 */ }
//	path := filepath.Join(dir, "hooks.json")
func CodexConfigDir(r CLIHomeResolver) (string, error) {
	dir, _, err := CodexConfigDirWithSource(r)
	return dir, err
}

// CodexHooksDisplayPath returns the human-readable display path for Codex
// CLI's hooks.json. When the resolved path is under homedir.Dir(), it is
// abbreviated with "~/" (e.g., "~/.codex/hooks.json" in the control case, or
// a custom subpath when CODEX_HOME points inside the home directory).
// When the resolved path is outside homedir.Dir() (e.g., CODEX_HOME=/some/dir
// or USERPROFILE ≠ HOME on Windows), the absolute path is returned as-is.
// Falls back to "~/.codex/hooks.json" when codexDir is empty (error path).
//
// ML-4A4 (R1): declared in agentconfig so both internal/generators (which uses
// it in codexHooksDisplayPath) and internal/validator (which cannot import
// generators due to circular dependency) can call the same function.
func CodexHooksDisplayPath(codexDir string) string {
	if codexDir == "" {
		return "~/.codex/hooks.json"
	}
	p := filepath.Join(codexDir, "hooks.json")
	home, err := homedir.Dir()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(home, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return "~/" + filepath.ToSlash(rel)
}

// InertCodexGuardPath returns (path, true) when HOME/.codex/hooks.json
// contains trackfw guard wiring but codexDir points elsewhere — indicating a
// D5 inert remainder (ADR-2026-10-14 D5, ML-4A4 R2).
//
// Returns ("", false) when:
//   - codexDir equals HOME/.codex (same location, no divergence);
//   - HOME/.codex/hooks.json does not exist or cannot be read;
//   - the file does not contain any trackfw guard wiring.
//
// The caller must NOT delete or modify the returned file — D5 is report-only.
//
// R-3 trade-off (ML-4A4, same as ADR-2026-10-14 for Claude): when CODEX_HOME
// is set but there is no Windows HOME≠USERPROFILE divergence, the guard at
// HOME/.codex is inert only for a Codex run that also sees CODEX_HOME. Without
// that variable, HOME/.codex IS the active guard. The D5 message is conditional
// — see withD5NoticeCodex in internal/generators/update.go.
func InertCodexGuardPath(codexDir string) (path string, present bool) {
	home, err := homedir.Dir()
	if err != nil {
		return "", false
	}
	oldDir := filepath.Join(home, ".codex")
	if strings.EqualFold(normHomePath(codexDir), normHomePath(oldDir)) {
		return "", false
	}
	p := filepath.Join(oldDir, "hooks.json")
	data, err := fileutil.ReadRegularFile(p)
	if err != nil {
		return "", false
	}
	if containsTrackfwGuardMarker(data) {
		return p, true
	}
	return "", false
}

// AntigravityConfigDir returns the directory that contains the Google Antigravity
// (agy) global hooks.json — `<home>/.gemini/config`, where `<home>` is resolved
// via HomeFor(CLIAntigravity).
//
// On Windows, HomeFor(CLIAntigravity) returns USERPROFILE (os.UserHomeDir()),
// matching the measurement in §12 of
// docs/portabilidade/2026-11-18-antigravity-contrato-de-hooks.md. On other
// platforms, it returns HOME via homedir.Dir().
//
// Callers join "hooks.json" to the result:
//
//	dir, err := agentconfig.AntigravityConfigDir(agentconfig.DefaultResolver)
//	if err != nil { /* emit finding per A7 */ }
//	path := filepath.Join(dir, "hooks.json")
func AntigravityConfigDir(r CLIHomeResolver) (string, error) {
	home, err := r.HomeFor(CLIAntigravity)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".gemini", "config"), nil
}

// AntigravityHooksDisplayPath returns the human-readable display path for the
// Google Antigravity global hooks.json. When the resolved path is under
// homedir.Dir(), it is abbreviated with "~/" (e.g., "~/.gemini/config/hooks.json"
// in the control case). When the resolved path is outside homedir.Dir()
// (e.g., USERPROFILE ≠ HOME on Windows), the absolute path is returned as-is.
// Falls back to "~/.gemini/config/hooks.json" when antigravityDir is empty.
//
// Declared in agentconfig so both internal/generators and internal/validator
// can call the same function (the validator cannot import generators due to
// circular dependency). Mirrors CodexHooksDisplayPath.
func AntigravityHooksDisplayPath(antigravityDir string) string {
	if antigravityDir == "" {
		return "~/.gemini/config/hooks.json"
	}
	p := filepath.Join(antigravityDir, "hooks.json")
	home, err := homedir.Dir()
	if err != nil {
		return p
	}
	rel, err := filepath.Rel(home, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return "~/" + filepath.ToSlash(rel)
}
