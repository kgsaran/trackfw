package validator_test

// validator_guard_wiring_concordance_external_test.go — ML-2B Q1 Lacuna 2 (REQ-2026-09-02)
//
// Concordance tests for guard_wiring_removed (validator_guard_wiring.go):
//
//  TestGuardWiringConcordance_AllGeneratorFormsRecognized — Part 1 (script-path legacy) +
//    Part 3 (sentinel). Asserts that every script-path legacy form in agentfiles.go is
//    recognised by guardWiringTypeFromCmd under at least one shell family. Floor: ≥ 9 forms.
//
//  TestGuardWiringConcordance_GeneratorOutputRecognized (ML-2B Lacuna 2a) — for each
//    Inject*Hooks that writes a guard (Claude, Codex, Gemini, Kiro, Copilot, Cursor,
//    Windsurf, Amazon Q), calls the generator in an isolated dir and asserts that every
//    guard command emitted by that generator is recognised by the tuple extractor of
//    guard_wiring_removed — floor declared per CLI.
//
//  TestGuardWiringConcordance_LegacyD2D11FormsRecognized (ML-2B Lacuna 2b) — scans
//    agentfiles.go for non-Global legacyD2* and legacyD11* constant definitions and asserts
//    each is recognised by guardWiringTypeFromCmd under the correct shell family. Floor: ≥ 6.
//
// Rationale: the validator cannot import generators (import cycle), so the equivalence
// class is maintained by hand. These tests bridge the gap — they fail automatically if:
//   (script-path) the generator gains a form not in guardWiringScriptPathForms;
//   (generator output) a generator emits a command not in the equivalence class;
//   (legacyD2/D11) a legacy inline constant is removed from the validator's comparison.
//
// Proof of bite — Lacuna 2a: removing guardExpectedLine from guardWiringTypeFromCmd causes
// all D11-revised forms emitted by the generators (PSPosix) to return "" → count = 0 →
// TestGuardWiringConcordance_GeneratorOutputRecognized fails for Claude/Codex/Gemini/
// Copilot/Cursor/Windsurf (PSPosix CLIs). Kiro and Amazon Q (CmdExe) are unaffected by
// that specific mutation because guardD11LegacyLine == guardExpectedLine for CmdExe;
// a second mutation removing both D11 clauses is needed to fail them.
//
// Proof of bite — Lacuna 2b: removing guardD2LegacyLine from guardWiringTypeFromCmd makes
// all D2-family entries (legacyD2*) unrecognized → TestGuardWiringConcordance_LegacyD2D11
// FormsRecognized fails for every D2 entry. TestGuardWiringConcordance_GeneratorOutput is
// unaffected because generators emit D11-revised, not D2.
//
// Regra de Reconciliação (CLAUDE.md):
// AllGeneratorFormsRecognized: afirma que toda forma de comando de guard no gerador (canônica
// + legado por script-path) é reconhecida por guardWiringTypeFromCmd.
// GeneratorOutputRecognized: afirma que cada Inject*Hooks emite comandos que a extração de
// tuplas de guard_wiring_removed reconhece — se o gerador ganhar forma fora da classe de
// equivalência, a contagem cai para 0 e o teste falha.
// LegacyD2D11FormsRecognized: afirma que as formas D2/D11 do gerador estão na classe de
// equivalência do validator para a família de shell correta.

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

// assertRecognized fails if neither PSPosix nor CmdExe family recognizes cmd.
func assertRecognized(t *testing.T, cmd, wantType string) {
	t.Helper()
	gotPS := validator.GuardWiringTypeFromCmdPSPosixForTest(cmd)
	gotCmd := validator.GuardWiringTypeFromCmdCmdExeForTest(cmd)
	if gotPS == wantType || gotCmd == wantType {
		return
	}
	t.Errorf("guard command not recognized (want %q):\n  cmd=%q\n  PSPosix→%q  CmdExe→%q",
		wantType, cmd, gotPS, gotCmd)
}

// agentFilesPath returns the absolute path to internal/generators/agentfiles.go,
// derived from the current test file's location via runtime.Caller.
func agentFilesPath(t *testing.T) string {
	t.Helper()
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// testFile is internal/validator/validator_guard_wiring_concordance_external_test.go
	validatorDir := filepath.Dir(testFile)
	p := filepath.Join(validatorDir, "..", "generators", "agentfiles.go")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("filepath.Abs(%q): %v", p, err)
	}
	return abs
}

// --------------------------------------------------------------------------
// Q1 concordance test
// --------------------------------------------------------------------------

// TestGuardWiringConcordance_AllGeneratorFormsRecognized verifies that every script-path
// legacy guard command form in agentfiles.go is recognised by guardWiringTypeFromCmd under
// at least one shell family, and that an unknown form is not recognized (sentinel).
//
// Reconciliação: afirma que toda forma de comando de guard script-path no gerador é
// reconhecida por guardWiringTypeFromCmd sob ao menos uma família de shell — se o gerador
// ganhar forma não coberta em guardWiringScriptPathForms, este teste falha com
// "form not recognized".
//
// The test has two parts (Part 2 is now TestGuardWiringConcordance_GeneratorOutputRecognized
// and TestGuardWiringConcordance_LegacyD2D11FormsRecognized):
//
//  1. Script-path legacy forms: scans agentfiles.go for lines containing
//     "trackfw-credential-guard.sh" or "trackfw-git-branch-guard.sh", extracts the string
//     constant values (handling the codexRoot concatenation), and asserts each is recognised.
//     Floor: at least 9 distinct script-path forms must be found (prevents vacuous pass).
//
//  2. Sentinel assertion: a form that the validator does NOT know ("echo scripts/...") must
//     return "" from both family variants (proves the helpers are wired to the real function,
//     not a stub that always returns a non-empty string).
func TestGuardWiringConcordance_AllGeneratorFormsRecognized(t *testing.T) {
	// -----------------------------------------------------------------------
	// Part 1: scan agentfiles.go for script-path forms
	// -----------------------------------------------------------------------
	const codexRootValue = `"$(git rev-parse --show-toplevel)`

	agentFile := agentFilesPath(t)
	raw, err := os.ReadFile(agentFile) //nolint:gosec // known project-internal path
	if err != nil {
		t.Fatalf("read agentfiles.go: %v", err)
	}

	// Extract all string literal values (single-quoted, double-quoted, raw backtick)
	// that contain a guard script path. We handle three patterns:
	//   A. `"some/path/trackfw-*.sh"` — regular string literal
	//   B. `` `some/path/trackfw-*.sh` `` — raw string literal
	//   C. `codexRoot + `/path/trackfw-*.sh"`` — concatenation with codexRoot
	const credSuffix = "trackfw-credential-guard.sh"
	const gitSuffix = "trackfw-git-branch-guard.sh"

	type scriptForm struct {
		cmd       string
		guardType string
	}
	var found []scriptForm
	seen := make(map[string]bool)

	addForm := func(cmd, guardType string) {
		if seen[cmd] {
			return
		}
		seen[cmd] = true
		found = append(found, scriptForm{cmd, guardType})
	}

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments.
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, "//") {
			continue
		}

		for _, pair := range []struct{ suffix, guardType string }{
			{credSuffix, "credential"},
			{gitSuffix, "git-branch"},
		} {
			if !strings.Contains(line, pair.suffix) {
				continue
			}
			// Determine if this is a codexRoot concatenation.
			isCodexConcat := strings.Contains(line, "codexRoot")

			// Extract the string value from the line.
			// Try raw string literal first (backtick), then regular string.
			var extracted string

			// Pattern C: codexRoot + `...`
			if isCodexConcat {
				// Find the raw string part after the `+`
				idx := strings.Index(line, "`")
				if idx >= 0 {
					end := strings.LastIndex(line, "`")
					if end > idx {
						rawPart := line[idx+1 : end]
						extracted = codexRootValue + rawPart
					}
				}
			}

			if extracted == "" {
				// Pattern B: raw string literal `` `...` ``
				idx := strings.Index(line, "`")
				if idx >= 0 {
					end := strings.LastIndex(line, "`")
					if end > idx {
						rawPart := line[idx+1 : end]
						if strings.Contains(rawPart, pair.suffix) {
							extracted = rawPart
						}
					}
				}
			}

			if extracted == "" {
				// Pattern A: regular string literal "..."
				// Find the first `"` after `=` and extract up to the next `"`.
				eqIdx := strings.Index(line, "=")
				if eqIdx < 0 {
					eqIdx = 0
				}
				sub := line[eqIdx:]
				dq := strings.Index(sub, `"`)
				if dq >= 0 {
					rest := sub[dq+1:]
					end := strings.Index(rest, `"`)
					if end >= 0 {
						val := rest[:end]
						if strings.Contains(val, pair.suffix) {
							extracted = val
						}
					}
				}
			}

			if extracted != "" && strings.Contains(extracted, pair.suffix) {
				addForm(extracted, pair.guardType)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning agentfiles.go: %v", err)
	}

	// Vacuity floor: at least 9 distinct script-path forms must be found.
	const minScriptForms = 9
	if len(found) < minScriptForms {
		t.Errorf("Q1 concordance vacuity: expected at least %d script-path forms in agentfiles.go, got %d; "+
			"the scan may be broken or the constants were removed",
			minScriptForms, len(found))
	}

	// Assert every found form is recognised.
	for _, f := range found {
		assertRecognized(t, f.cmd, f.guardType)
	}

	// -----------------------------------------------------------------------
	// Part 2: sentinel — an unknown form must return "" (non-vacuity of the helpers)
	// -----------------------------------------------------------------------
	// (Canonical / D11-legacy / D2-legacy inline forms are now tested in separate
	// top-level functions TestGuardWiringConcordance_GeneratorOutputRecognized and
	// TestGuardWiringConcordance_LegacyD2D11FormsRecognized.)
	sentinel := "echo scripts/trackfw-credential-guard.sh"
	if got := validator.GuardWiringTypeFromCmdPSPosixForTest(sentinel); got != "" {
		t.Errorf("sentinel: unknown form must return \"\"; got %q for %q", got, sentinel)
	}
	if got := validator.GuardWiringTypeFromCmdCmdExeForTest(sentinel); got != "" {
		t.Errorf("sentinel: unknown form must return \"\"; got %q for %q", got, sentinel)
	}
}

// --------------------------------------------------------------------------
// ML-2B Lacuna 2a — generator output concordance
// --------------------------------------------------------------------------

// TestGuardWiringConcordance_GeneratorOutputRecognized verifies that for each Inject*Hooks
// function that writes guard commands, the commands emitted are recognised by the tuple
// extractor of guard_wiring_removed (via GuardWiringExtractGuardTypesFromFileForTest which
// calls guardWiringParseTuples with the canonical family from credentialGuardHookFiles).
//
// Reconciliação: afirma que cada Inject*Hooks emite, no arquivo de hook correto, ao menos
// uma tupla de credential-guard e uma de git-branch-guard reconhecidas pela regra
// guard_wiring_removed — se o gerador mudar o formato do comando para fora da classe de
// equivalência do validator, a contagem cai para 0 e o sub-teste falha.
//
// Piso por CLI (floor declared here; must be ≥ 1 per guard type):
//
//	Claude, Codex, Gemini, Kiro, Copilot, Cursor, Windsurf, Amazon Q:
//	  credential ≥ 1, git-branch ≥ 1
//
// (Windsurf and Amazon Q write credential guard when the global harness is not installed;
// t.Setenv("HOME", t.TempDir()) in setupConcordanceDir ensures the global harness is absent.)
func TestGuardWiringConcordance_GeneratorOutputRecognized(t *testing.T) {
	type cliCase struct {
		cli      string
		hookPath string // relative path — must be in credentialGuardHookFiles
		inject   func(string) error
		minCred  int
		minGit   int
	}
	cases := []cliCase{
		{"Claude", ".claude/settings.json", generators.InjectClaudeHooks, 1, 1},
		{"Codex", ".codex/hooks.json", generators.InjectCodexHooks, 1, 1},
		{"Gemini", ".gemini/settings.json", generators.InjectGeminiHooks, 1, 1},
		{"Kiro", ".kiro/hooks/trackfw-attention.json", generators.InjectKiroHooks, 1, 1},
		{"Copilot", ".github/hooks/trackfw-attention.json", generators.InjectCopilotHooks, 1, 1},
		{"Cursor", ".cursor/hooks.json", generators.InjectCursorHooks, 1, 1},
		{"Windsurf", ".windsurf/hooks.json", generators.InjectWindsurfHooks, 1, 1},
		{"AmazonQ", ".amazonq/cli-agents/q_cli_default.json", generators.InjectAmazonQHooks, 1, 1},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.cli, func(t *testing.T) {
			dir := setupConcordanceDir(t)
			if err := tc.inject(dir); err != nil {
				t.Fatalf("%s: inject: %v", tc.cli, err)
			}

			// GuardWiringExtractGuardTypesFromFileForTest reads the file relative to CWD
			// (which is dir after setupConcordanceDir), looks up the canonical family from
			// credentialGuardHookFiles, and calls guardWiringParseTuples.
			guardTypes, err := validator.GuardWiringExtractGuardTypesFromFileForTest(tc.hookPath)
			if err != nil {
				t.Fatalf("%s: extract guard types from %q: %v", tc.cli, tc.hookPath, err)
			}

			counts := make(map[string]int)
			for _, gt := range guardTypes {
				counts[gt]++
			}
			if counts["credential"] < tc.minCred {
				t.Errorf("%s: credential guard count: want >= %d, got %d (all types: %v)",
					tc.cli, tc.minCred, counts["credential"], guardTypes)
			}
			if counts["git-branch"] < tc.minGit {
				t.Errorf("%s: git-branch guard count: want >= %d, got %d (all types: %v)",
					tc.cli, tc.minGit, counts["git-branch"], guardTypes)
			}
		})
	}
}

// --------------------------------------------------------------------------
// ML-2B Lacuna 2b — legacyD2* and legacyD11* inline constants
// --------------------------------------------------------------------------

// TestGuardWiringConcordance_LegacyD2D11FormsRecognized scans agentfiles.go for non-Global
// legacyD2* and legacyD11* constant definitions and asserts each is recognised by
// guardWiringTypeFromCmd under the correct shell family (PSPosix for constants with "PSPOSIX"
// in the name, CmdExe for "CmdExe").
//
// Reconciliação: afirma que as formas D2-legacy e D11-legacy no gerador estão na classe de
// equivalência do validator para a família de shell correta — se guardD2LegacyLine ou
// guardD11LegacyLine for removido de guardWiringTypeFromCmd, as asserções das constantes D2
// ou D11 falham.
//
// Floor: at least 6 distinct non-Global legacyD2*/legacyD11* constants found (prevents
// vacuous pass if the scan pattern breaks).
func TestGuardWiringConcordance_LegacyD2D11FormsRecognized(t *testing.T) {
	agentFile := agentFilesPath(t)
	raw, err := os.ReadFile(agentFile) //nolint:gosec
	if err != nil {
		t.Fatalf("read agentfiles.go: %v", err)
	}

	type legacyForm struct {
		name      string
		cmd       string
		guardType string // "credential" or "git-branch"
		family    string // "ps" or "cmd"
	}
	var found []legacyForm
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(strings.NewReader(string(raw)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip comments.
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Only lines that define a legacyD2* or legacyD11* constant.
		if !strings.Contains(trimmed, "legacyD2") && !strings.Contains(trimmed, "legacyD11") {
			continue
		}
		// Skip Global variants (those are global-harness commands, not project-scope).
		if strings.Contains(trimmed, "Global") {
			continue
		}
		// Must contain "=" (constant definition, not a function call argument).
		eqIdx := strings.Index(trimmed, "=")
		if eqIdx < 0 {
			continue
		}
		constName := strings.TrimSpace(trimmed[:eqIdx])

		// Accept only names that start with legacyD2 or legacyD11.
		if !strings.HasPrefix(constName, "legacyD2") && !strings.HasPrefix(constName, "legacyD11") {
			continue
		}

		// Determine guard type from constant name.
		var guardType string
		switch {
		case strings.Contains(constName, "GitBranch") || strings.Contains(constName, "GitGuard"):
			guardType = "git-branch"
		case strings.Contains(constName, "Credential") || strings.Contains(constName, "CredGuard"):
			guardType = "credential"
		default:
			continue // unrecognised guard type — skip
		}

		// Determine shell family from constant name.
		var family string
		switch {
		case strings.Contains(constName, "PSPOSIX") || strings.Contains(constName, "PSPosix"):
			family = "ps"
		case strings.Contains(constName, "CmdExe"):
			family = "cmd"
		default:
			continue // unrecognised family — skip
		}

		// Extract the string value (raw backtick or double-quoted).
		rest := strings.TrimSpace(trimmed[eqIdx+1:])
		var value string
		if strings.HasPrefix(rest, "`") {
			inner := rest[1:]
			end := strings.Index(inner, "`")
			if end < 0 {
				continue
			}
			value = inner[:end]
		} else if strings.HasPrefix(rest, `"`) {
			inner := rest[1:]
			end := strings.Index(inner, `"`)
			if end < 0 {
				continue
			}
			value = inner[:end]
		} else {
			continue
		}

		if value == "" || seen[value+"__"+family] {
			continue
		}
		seen[value+"__"+family] = true
		found = append(found, legacyForm{constName, value, guardType, family})
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning agentfiles.go: %v", err)
	}

	// Vacuity floor: at least 6 non-Global D2/D11 forms must be found.
	const minLegacyForms = 6
	if len(found) < minLegacyForms {
		t.Errorf("legacyD2/D11 concordance vacuity: expected >= %d non-Global forms, got %d "+
			"(Global constants are intentionally excluded; scan pattern may be broken)",
			minLegacyForms, len(found))
	}

	// Assert each is recognized under the correct shell family (not either-family).
	for _, f := range found {
		var got string
		if f.family == "ps" {
			got = validator.GuardWiringTypeFromCmdPSPosixForTest(f.cmd)
		} else {
			got = validator.GuardWiringTypeFromCmdCmdExeForTest(f.cmd)
		}
		if got != f.guardType {
			t.Errorf("constant %s (family=%s) not recognized: cmd=%q → %q, want %q",
				f.name, f.family, f.cmd, got, f.guardType)
		}
	}
}
