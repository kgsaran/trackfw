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
	"os"
	"path/filepath"
	"runtime"

	"github.com/kgsaran/trackfw/internal/homedir"
)

// CLI name constants used by HomeFor and ClaudeConfigDir.
const (
	CLIClaude   = "claude"
	CLICodex    = "codex"
	CLIGemini   = "gemini"
	CLICursor   = "cursor"
	CLICopilot  = "copilot"
	CLIWindsurf = "windsurf"
	CLIKiro     = "kiro"
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
	if cli == CLIClaude && effectiveGOOS() == "windows" {
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

// ClaudeConfigDir returns the directory that contains Claude Code's settings.json.
//
// Resolution order (A1, ADR-2026-10-14 D6):
//
//  1. If CLAUDE_CONFIG_DIR is set and non-empty: returns filepath.Clean(CLAUDE_CONFIG_DIR).
//     settings.json lives at the ROOT of this directory (no .claude/ subdir) — measured
//     2026-10-14 on Windows ARM64 (docs/portabilidade/2026-10-14-guard-global-home-userprofile-vm.md
//     Passo 3.1). This behaviour is cross-platform.
//
//  2. Otherwise: filepath.Join(r.HomeFor(CLIClaude), ".claude").
//     On Windows (via OSCLIHomeResolver) this uses USERPROFILE; on other platforms, HOME.
//
// Callers join "settings.json" to the result:
//
//	dir, err := agentconfig.ClaudeConfigDir(agentconfig.DefaultResolver)
//	if err != nil { /* emit finding per A7 */ }
//	path := filepath.Join(dir, "settings.json")
func ClaudeConfigDir(r CLIHomeResolver) (string, error) {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return filepath.Clean(dir), nil
	}
	home, err := r.HomeFor(CLIClaude)
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude"), nil
}
