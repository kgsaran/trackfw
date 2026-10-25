package validator_test

// validator_guard_d16_wiring_external_test.go — ML-1A2 (REQ-2026-10-25, D16)
//
// Regra Dura de Reconciliação (CLAUDE.md): cada teste declara em um comentário prefixado
// "Reconciliação:" a conclusão do ML que afirma — em uma frase.
//
// Tests in this file exercise two AC4 scenarios that require either a real git repository
// (guard_wiring_removed via ValidateGuardWiringRemovedForTest) or the public generator
// API (Amazon Q fail-open via InjectAmazonQHooks).
//
// Item 1: guard_wiring_removed — table test with real git repo + bare origin.
// Item 2: Amazon Q fail-open (marker in "note" field, not "command") and Kiro.

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kgsaran/trackfw/internal/generators"
	"github.com/kgsaran/trackfw/internal/validator"
)

// --------------------------------------------------------------------------
// git helpers (package-local, no conflict with internal helpers)
// --------------------------------------------------------------------------

func d16RunGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %s", args, dir, string(out))
	}
}

// d16InitGitRepo creates a minimal git repository with one empty commit on branch "main".
// GIT_CONFIG_PARAMETERS suppresses maintenance auto-scheduling for the subprocess chain.
func d16InitGitRepo(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("GIT_CONFIG_PARAMETERS", "'gc.auto=0' 'maintenance.auto=false'")
	d16RunGit(t, dir, "init", "-b", "main")
	d16RunGit(t, dir, "config", "user.email", "test@test.com")
	d16RunGit(t, dir, "config", "user.name", "test")
}

// d16SetupOriginWithFiles creates a bare origin repo, commits the given files into it,
// and adds it as "origin" to workDir (fetching origin/main).
func d16SetupOriginWithFiles(t *testing.T, workDir string, files map[string]string) {
	t.Helper()
	bareDir := t.TempDir()
	d16RunGit(t, bareDir, "init", "--bare", bareDir)

	tmpClone := t.TempDir()
	d16RunGit(t, tmpClone, "init")
	d16RunGit(t, tmpClone, "config", "user.email", "test@test.com")
	d16RunGit(t, tmpClone, "config", "user.name", "test")
	for rel, content := range files {
		p := filepath.Join(tmpClone, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			t.Fatalf("writeFile %q: %v", rel, err)
		}
		d16RunGit(t, tmpClone, "add", rel)
	}
	d16RunGit(t, tmpClone, "commit", "--allow-empty", "-m", "init")
	d16RunGit(t, tmpClone, "remote", "add", "origin", bareDir)
	d16RunGit(t, tmpClone, "push", "origin", "HEAD:main")

	d16RunGit(t, workDir, "remote", "add", "origin", bareDir)
	d16RunGit(t, workDir, "fetch", "origin")
}

// --------------------------------------------------------------------------
// Item 1: guard_wiring_removed table test
// --------------------------------------------------------------------------

// d16WiringCase describes one row in the guard_wiring_removed table test.
type d16WiringCase struct {
	cli    string
	inject func(dir string) error
	// hookPath is the project-relative path of the hook file the generator writes.
	hookPath string
}

// d16WiringCases lists the CLIs whose generator writes a project-scope hook file that
// guard_wiring_removed checks. Order follows credentialGuardHookFiles.
var d16WiringCases = []d16WiringCase{
	{"Claude Code", generators.InjectClaudeHooks, ".claude/settings.json"},
	{"Codex CLI", generators.InjectCodexHooks, ".codex/hooks.json"},
	{"Gemini CLI", generators.InjectGeminiHooks, ".gemini/settings.json"},
	{"Cursor", generators.InjectCursorHooks, ".cursor/hooks.json"},
	{"GitHub Copilot CLI", generators.InjectCopilotHooks, ".github/hooks/trackfw-attention.json"},
	{"Kiro", generators.InjectKiroHooks, ".kiro/hooks/trackfw-attention.json"},
	{"Windsurf", generators.InjectWindsurfHooks, ".windsurf/hooks.json"},
	{"Amazon Q", generators.InjectAmazonQHooks, ".amazonq/cli-agents/q_cli_default.json"},
}

// TestGuardWiringRemoved_ControlArm_OrigemIntacta afirma: quando o disco tem o mesmo conteúdo
// que origin/main (saída do gerador, sem mutação), validateGuardWiringRemoved não emite
// nenhuma mensagem de "is absent from disk" — arm de controle.
//
// Reconciliação: guard_wiring_removed (ADR-2026-10-10 ML-1A) já usava guardWiringExtractCommandsFromEntry
// (schema-aware, R-1 ML-2C) antes deste ML; com o disco intacto, nenhuma tupla de fiação pode estar
// ausente — este arm prova que o git setup funciona e não há falso positivo de configuração.
//
// Nota: este teste é um pin de regressão, não uma prova de mordida de D16. O advisor confirmou
// que guardWiringExtractCommandsFromEntry já era schema-aware antes de ML-1A2; a propriedade
// arm-de-controle=0 vale também em origin/main.
func TestGuardWiringRemoved_ControlArm_OrigemIntacta(t *testing.T) {
	for _, tc := range d16WiringCases {
		tc := tc
		t.Run(tc.cli, func(t *testing.T) {
			// No t.Parallel(): t.Setenv and t.Chdir are not allowed in parallel subtests.

			dir := t.TempDir()
			t.Setenv("HOME", t.TempDir()) // isolate global harness dedup
			d16InitGitRepo(t, dir)
			validator.StubProbeOKForTest(t)

			// Generate the hook file in a separate temp dir to capture origin content.
			genDir := t.TempDir()
			if err := tc.inject(genDir); err != nil {
				t.Fatalf("inject(%s): %v", tc.cli, err)
			}
			hookContent, err := os.ReadFile(filepath.Join(genDir, tc.hookPath))
			if err != nil {
				// Generator may not write a file for all CLIs in all configurations.
				t.Skipf("generator did not write %s for %s: %v", tc.hookPath, tc.cli, err)
			}

			// Set up origin/main with the generated content.
			d16SetupOriginWithFiles(t, dir, map[string]string{
				tc.hookPath: string(hookContent),
			})

			// Copy the same content to disk (control: disk == origin/main).
			diskPath := filepath.Join(dir, tc.hookPath)
			if mkErr := os.MkdirAll(filepath.Dir(diskPath), 0755); mkErr != nil {
				t.Fatalf("mkdir: %v", mkErr)
			}
			if writeErr := os.WriteFile(diskPath, hookContent, 0644); writeErr != nil {
				t.Fatalf("write disk: %v", writeErr)
			}

			validator.ChdirForTest(t, dir)
			msgs, err := validator.ValidateGuardWiringRemovedForTest()
			if err != nil {
				t.Fatalf("ValidateGuardWiringRemoved: %v", err)
			}
			for _, m := range msgs {
				if strings.Contains(m, "is absent from disk") {
					t.Errorf("control arm: unexpected 'is absent from disk' for %s: %s", tc.cli, m)
				}
			}
		})
	}
}

// TestGuardWiringRemoved_MutacaoDoComandoParaEchoX afirma: quando o disco tem o arquivo de hook
// com o comando do guard substituído por "echo x" (e o original movido para "note"), a regra
// guard_wiring_removed emite "is absent from disk" para aquele arquivo.
//
// Reconciliação: D16 schema-aware collection via guardWiringExtractCommandsFromEntry — o guard
// marker no campo "note" (ou "action.note" no Kiro) não conta como fiação; a tupla de origin/main
// não é coberta pelas tuplas do disco mutado, logo guard_wiring_removed dispara.
//
// Prova de mordida: no binário origin/main, guardWiringExtractCommandsFromEntry já era
// schema-aware; este arm também passa em origin/main (pin de regressão). A prova de mordida de
// D16 está no arm de presença (TestCredentialGuardPresenceRequired_Windsurf_MarkerSoEmNote_Violation,
// validator_credential_guard_test.go) e no arm do falso positivo (AC3).
func TestGuardWiringRemoved_MutacaoDoComandoParaEchoX(t *testing.T) {
	for _, tc := range d16WiringCases {
		tc := tc
		t.Run(tc.cli, func(t *testing.T) {
			// No t.Parallel(): t.Setenv and t.Chdir are not allowed in parallel subtests.

			dir := t.TempDir()
			t.Setenv("HOME", t.TempDir())
			d16InitGitRepo(t, dir)
			validator.StubProbeOKForTest(t)

			genDir := t.TempDir()
			if err := tc.inject(genDir); err != nil {
				t.Fatalf("inject(%s): %v", tc.cli, err)
			}
			hookContent, err := os.ReadFile(filepath.Join(genDir, tc.hookPath))
			if err != nil {
				t.Skipf("generator did not write %s for %s: %v", tc.hookPath, tc.cli, err)
			}

			d16SetupOriginWithFiles(t, dir, map[string]string{
				tc.hookPath: string(hookContent),
			})

			// Mutate the disk copy: replace the guard command in the command field with
			// "echo x" and stash the original in "note" / "action.note".
			mutated, hadGuard := d16MutateCommandToEchoX(t, hookContent)
			if !hadGuard {
				// The generator did not write a guard command for this CLI in this
				// configuration — wiring_removed has nothing to compare.
				t.Skipf("generator output for %s has no guard command; skip mutation arm", tc.cli)
			}

			diskPath := filepath.Join(dir, tc.hookPath)
			if mkErr := os.MkdirAll(filepath.Dir(diskPath), 0755); mkErr != nil {
				t.Fatalf("mkdir: %v", mkErr)
			}
			if writeErr := os.WriteFile(diskPath, mutated, 0644); writeErr != nil {
				t.Fatalf("write mutated disk: %v", writeErr)
			}

			validator.ChdirForTest(t, dir)
			msgs, err := validator.ValidateGuardWiringRemovedForTest()
			if err != nil {
				t.Fatalf("ValidateGuardWiringRemoved: %v", err)
			}
			found := false
			for _, m := range msgs {
				if strings.Contains(m, tc.hookPath) && strings.Contains(m, "is absent from disk") {
					found = true
					break
				}
			}
			if !found {
				t.Errorf(
					"AC4 (wiring_removed): expected 'is absent from disk' for %s (%s) after "+
						"command replaced with 'echo x' and original moved to note; msgs=%v",
					tc.hookPath, tc.cli, msgs,
				)
			}
		})
	}
}

// d16MutateCommandToEchoX parses the hook JSON content and replaces every guard command string
// in the executable-command field with "echo x", moving the original to a "note" sibling field.
// Returns (mutated JSON bytes, true) if at least one command was replaced; (original, false) otherwise.
//
// The mutation is schema-agnostic at the JSON level: it replaces any string value that contains
// a guard marker (credentialGuardScriptMarker or gitBranchGuardScriptMarker) anywhere in the
// entire document — in the command field only. Because this function is test-only and we want to
// test that the validator does NOT recognise the original command once it is moved to "note", we
// use a shallow walk: replace "command" key values that contain a guard marker, and add a "note"
// sibling. This is the INVERSE of the validator's schema-aware restriction.
func d16MutateCommandToEchoX(t *testing.T, content []byte) ([]byte, bool) {
	t.Helper()
	var root interface{}
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatalf("d16MutateCommandToEchoX: unmarshal: %v", err)
	}
	changed := false
	d16mutateValue(&root, &changed)
	if !changed {
		return content, false
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		t.Fatalf("d16MutateCommandToEchoX: marshal: %v", err)
	}
	return out, true
}

// guardMarkers are the strings whose presence in a "command"/"bash"/"action.command" value
// indicates a guard wiring entry that should be mutated for the test.
var guardMarkersForMutation = []string{
	"trackfw-credential-guard.sh",
	"trackfw-git-branch-guard.sh",
	"trackfw guard credential",
	"trackfw guard git-branch",
}

func d16isGuardCmd(s string) bool {
	for _, m := range guardMarkersForMutation {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// d16mutateValue recurses through the JSON tree and, for any map object,
// replaces the value of "command", "bash" keys that contain a guard marker
// with "echo x", and adds a "note" sibling with the original value.
// For Kiro "action" objects, the same mutation is applied to action["command"].
func d16mutateValue(v *interface{}, changed *bool) {
	switch t := (*v).(type) {
	case map[string]interface{}:
		// Check "command" key.
		if cmd, ok := t["command"].(string); ok && d16isGuardCmd(cmd) {
			t["note"] = cmd
			t["command"] = "echo x"
			*changed = true
		}
		// Check "bash" key (Copilot global schema).
		if cmd, ok := t["bash"].(string); ok && d16isGuardCmd(cmd) {
			t["note"] = cmd
			t["bash"] = "echo x"
			*changed = true
		}
		// Recurse into children.
		for k, val := range t {
			d16mutateValue(&val, changed)
			t[k] = val
		}
	case []interface{}:
		for i := range t {
			d16mutateValue(&t[i], changed)
		}
	}
}

// --------------------------------------------------------------------------
// Item 2: Amazon Q fail-open (AC4 — disk-only presence rule)
// --------------------------------------------------------------------------

// d16AmazonQHooksWithNoteOnlyMarker builds a .amazonq/cli-agents/q_cli_default.json
// where the credential-guard expected line appears only in the "note" field of an inner
// hook entry — NOT in the "command" field — for both required matchers.
// Used to prove the schema-aware search (D16) does not count non-command fields as wiring.
func d16AmazonQHooksWithNoteOnlyMarker(guardLine string) string {
	entry := func(matcher string) string {
		return `{
      "matcher": "` + matcher + `",
      "hooks": [
        {
          "type": "command",
          "command": "echo placeholder",
          "note": "` + guardLine + `"
        }
      ]
    }`
	}
	return `{
  "hooks": {
    "preToolUse": [
      ` + entry("execute_bash") + `,
      ` + entry("fs_write") + `
    ]
  }
}`
}

// TestCredentialGuardPresenceRequired_AmazonQ_MarkerSoEmNote_Violation — AC4 (D16, fail-open Amazon Q)
//
// Reconciliação: D16 schema-aware search via extractCredentialGuardCommandsFromEvent /
// collectCommandsFromCommandKeyArray — the credential-guard expected line placed in a "note"
// field of an Amazon Q inner hook entry does NOT count as wiring; the presence rule
// emits violations for both required matchers (execute_bash, fs_write).
//
// Prova de mordida: the old collectCommandsWithMarker traversed all JSON strings; a marker
// in "note" satisfied the check → 0 presence violations (fail-open). The schema-aware version
// (collectCommandsFromCommandKeyArray) only reads "command" keys in the inner hooks array.
func TestCredentialGuardPresenceRequired_AmazonQ_MarkerSoEmNote_Violation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	// Use the canonical expected line for the cmd.exe family (Amazon Q family).
	guardLine := validator.GuardExpectedLineCmdExeForTest("credential")

	hookPath := filepath.Join(dir, ".amazonq", "cli-agents", "q_cli_default.json")
	if err := os.MkdirAll(filepath.Dir(hookPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(hookPath, []byte(d16AmazonQHooksWithNoteOnlyMarker(guardLine)), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	msgs, err := validator.ValidateCredentialGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Expect violations for both matchers — marker in "note" must not count as wiring.
	for _, matcher := range []string{"execute_bash", "fs_write"} {
		found := false
		for _, m := range msgs {
			if strings.Contains(m, ".amazonq") && strings.Contains(m, "preToolUse["+matcher+"]") {
				found = true
				break
			}
		}
		if !found {
			t.Errorf(
				"AC4 Amazon Q: expected presence violation for preToolUse[%s] when guard marker "+
					"is only in 'note' field; got: %v", matcher, msgs,
			)
		}
	}
}

// TestCredentialGuardPresenceRequired_Kiro_NaoTemRegraDePresencaDisco afirma: Kiro não está em
// credentialGuardRequiredEntries — sua única regra de presença é guard_wiring_removed (que compara
// com origin/main). Nenhum teste de fail-open por disk-only rule é necessário; o fail-open de
// guard_wiring_removed é exercido pelos testes item-1 acima.
//
// Reconciliação: Kiro não tem regra de presença disk-only em validateCredentialGuardPresenceRequired;
// declarar isso explicitamente impede que um leitor futuro interprete a ausência de teste como
// lacuna e adicione incorretamente Kiro a credentialGuardRequiredEntries.
func TestCredentialGuardPresenceRequired_Kiro_NaoTemRegraDePresencaDisco(t *testing.T) {
	// This test is intentionally documentation-only (no assertions about runtime behavior).
	// It verifies that InjectKiroHooks generates a hook file and that calling the
	// credential-guard presence rule on that file emits no disk-only presence violations
	// (because Kiro is not in credentialGuardRequiredEntries).
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	if err := generators.InjectKiroHooks(dir); err != nil {
		t.Fatalf("InjectKiroHooks: %v", err)
	}

	msgs, err := validator.ValidateCredentialGuardHookResolvableForTest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Kiro is not in credentialGuardRequiredEntries; no disk-only presence violation should fire.
	for _, m := range msgs {
		if strings.Contains(m, ".kiro") && strings.Contains(m, "has no credential guard entry") {
			t.Errorf(
				"Kiro disk-only presence violation fired unexpectedly — Kiro should not be in "+
					"credentialGuardRequiredEntries; got: %s", m,
			)
		}
	}
}

// --------------------------------------------------------------------------
// Item 5: Windsurf global — measurement with isolated HOME
// --------------------------------------------------------------------------

// d16WindsurfGlobalWithCurrentForm builds ~/.codeium/windsurf/hooks.json with the canonical
// current-form credential guard command in both required events.
func d16WindsurfGlobalWithCurrentForm(guardCmd string) []byte {
	entry := map[string]interface{}{
		"command":     guardCmd,
		"show_output": true,
	}
	root := map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{entry},
			"pre_write_code":  []interface{}{entry},
		},
	}
	out, _ := json.MarshalIndent(root, "", "  ")
	return out
}

// d16WindsurfGlobalWithLegacyScript builds ~/.codeium/windsurf/hooks.json with the legacy
// .sh script form in both required events (simulates pre-D11 harness).
func d16WindsurfGlobalWithLegacyScript(scriptPath string) []byte {
	entry := map[string]interface{}{
		"command":     scriptPath,
		"show_output": true,
	}
	root := map[string]interface{}{
		"hooks": map[string]interface{}{
			"pre_run_command": []interface{}{entry},
			"pre_write_code":  []interface{}{entry},
		},
	}
	out, _ := json.MarshalIndent(root, "", "  ")
	return out
}

// TestGuardGlobalHookResolvable_Windsurf_CurrentForm_ZeroViolation — item 5 measurement
//
// Reconciliação: Windsurf global (~/.codeium/windsurf/hooks.json) was absent from
// globalGuardConfigFiles before ML-1A2; adding it (with hookCmdSchemaFlat) means the validator
// now reads the file and correctly recognises a current-form credential guard entry — zero
// violations for a well-formed Windsurf global harness.
func TestGuardGlobalHookResolvable_Windsurf_CurrentForm_ZeroViolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "") // ensure Claude path doesn't bleed into HOME

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	// Write a ~/.codeium/windsurf/hooks.json with the current-form command.
	guardCmd := validator.GuardExpectedLinePSPosixForTest("credential --global")
	globalDir := filepath.Join(home, ".codeium", "windsurf")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir windsurf global: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(globalDir, "hooks.json"),
		d16WindsurfGlobalWithCurrentForm(guardCmd), 0644); err != nil {
		t.Fatalf("write windsurf global: %v", err)
	}

	msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range msgs {
		if strings.Contains(m, ".codeium/windsurf") {
			t.Errorf(
				"item 5: unexpected violation for Windsurf global with current-form command: %s", m,
			)
		}
	}
}

// TestGuardGlobalHookResolvable_Windsurf_MissingScript_EmitsViolation — item 5 measurement (missing-script arm)
//
// Reconciliação: Windsurf global (~/.codeium/windsurf/hooks.json) is now in globalGuardConfigFiles;
// the schema-aware collection (hookCmdSchemaFlat) finds the guard script marker in the "command"
// field and emits a violation when the referenced script file is absent — proving the file is READ.
//
// Before ML-1A2 Windsurf global was absent from globalGuardConfigFiles; this test would silently
// return 0 messages (file never read). After the addition, the "script not found" violation fires.
func TestGuardGlobalHookResolvable_Windsurf_MissingScript_EmitsViolation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("CLAUDE_CONFIG_DIR", "")

	dir := t.TempDir()
	validator.ChdirForTest(t, dir)
	validator.StubProbeOKForTest(t)

	// Write ~/.codeium/windsurf/hooks.json with a legacy .sh command pointing to a
	// NON-EXISTENT script. The resolvable rule must emit a violation.
	missingScript := filepath.Join(home, ".trackfw", "scripts", "trackfw-credential-guard.sh")
	// Do NOT create the script — the test relies on it being absent.

	globalDir := filepath.Join(home, ".codeium", "windsurf")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("mkdir windsurf global: %v", err)
	}
	if err := os.WriteFile(
		filepath.Join(globalDir, "hooks.json"),
		d16WindsurfGlobalWithLegacyScript(missingScript), 0644); err != nil {
		t.Fatalf("write windsurf global: %v", err)
	}

	msgs, err := validator.ValidateCredentialGuardGlobalHookResolvableForTest()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expect a violation mentioning the Windsurf global file.
	windsurfViolation := false
	for _, m := range msgs {
		if strings.Contains(m, ".codeium/windsurf") || strings.Contains(m, "Windsurf") {
			windsurfViolation = true
			break
		}
	}
	if !windsurfViolation {
		t.Errorf(
			"item 5 missing-script arm: expected violation mentioning Windsurf global "+
				"(.codeium/windsurf) when script is absent — file must now be read via "+
				"globalGuardConfigFiles addition; got no such message in: %v", msgs,
		)
	}
}
