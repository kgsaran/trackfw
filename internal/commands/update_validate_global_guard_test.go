package commands

// update_validate_global_guard_test.go — AC9 / D15 (ADR-2026-10-04 adendo 2026-10-24)
//
// D15 contract: project-scope guard wiring is required regardless of whether the
// global harness guard is also installed on the machine. The prior D14 short-circuit
// that skipped project-scope writes when a global guard was present has been removed.
//
// This file contains two test functions:
//
//   TestD15_UpdatePreservesProjectGuardWithGlobalInstalled — table-driven over
//   Claude, Codex, Gemini, Cursor, Copilot, Kiro and Windsurf.
//   For each CLI: initialise a project, commit hook files to origin/main, install
//   the global harness guard, run `trackfw update` (project scope), then run
//   `trackfw validate --json` and assert rc=0 with zero guard_wiring_removed
//   violations. Also asserts the hook file still contains "trackfw guard credential".
//
//   Reconciliation: affirms that the D15 per-CLI update path does NOT remove the
//   project-scope credential guard when the global guard is installed — the exact
//   defect reported in issue #613 (guard_wiring_removed fires after `trackfw update`
//   when global guard is present; Claude, Codex and Gemini reproved on origin/main).
//
//   Falsification contract: re-introduce the D14 global short-circuit in any of
//   the per-CLI update inject functions (condition: "global installed → skip project
//   wiring") → hook file loses the credential guard → validate fires
//   guard_wiring_removed → that CLI's subtest fails.
//
//   TestD15_GlobalOnlyDoesNotSatisfyProjectScope — ArmB.
//   With the global guard installed but the project hook file rewritten WITHOUT the
//   credential guard, `trackfw validate` must report a violation.
//
//   Reconciliation: affirms that the D14 global short-circuit in
//   validateCredentialGuardPresenceRequired is gone — global installation alone does
//   not satisfy the project-scope requirement (D15 requires the guard in the project
//   file regardless of global state).
//
//   Falsification contract: re-introduce the global short-circuit in
//   validateCredentialGuardPresenceRequired → validate returns nil on global-only →
//   ArmB fails with "validate should fail".

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// d15Row defines one CLI under test with its harness target IDs.
// harness is the list of IDs passed to `trackfw update harness --targets`.
// Windsurf has no global git-branch-guard target; the others have both.
type d15Row struct {
	cli     string
	harness []string
}

// d15Rows is the closed list of CLIs that have at least one global harness guard.
// Claude, Codex and Gemini are first: they are the CLIs that reproved on origin/main
// in the contra-arm measurement (the D14 skip was active for them).
var d15Rows = []d15Row{
	{"claude", []string{"claude-credential-guard", "claude-git-branch-guard"}},
	{"codex", []string{"codex-credential-guard", "codex-git-branch-guard"}},
	{"gemini", []string{"gemini-credential-guard", "gemini-git-branch-guard"}},
	{"cursor", []string{"cursor-credential-guard", "cursor-git-branch-guard"}},
	{"copilot", []string{"copilot-credential-guard", "copilot-git-branch-guard"}},
	{"kiro", []string{"kiro-credential-guard", "kiro-git-branch-guard"}},
	{"windsurf", []string{"windsurf-credential-guard"}},
}

// d15CleanEnv builds an isolated subprocess env based on e2eCleanEnv.
// It strips any CLAUDE_CONFIG_DIR that leaked from the OS env, pins it to
// fakeHome/.claude, and prepends binDir to PATH so guard_binary_probe finds
// the compiled trackfw binary.
func d15CleanEnv(fakeHome, binDir string) []string {
	base := e2eCleanEnv(fakeHome)
	result := make([]string, 0, len(base)+2)
	pathHandled := false
	for _, kv := range base {
		k := strings.SplitN(kv, "=", 2)[0]
		switch k {
		case "PATH":
			if !pathHandled {
				v := strings.SplitN(kv, "=", 2)[1]
				result = append(result, "PATH="+binDir+string(os.PathListSeparator)+v)
				pathHandled = true
			}
			// skip duplicate PATH entries that e2eCleanEnv may produce
		case "CLAUDE_CONFIG_DIR":
			// strip leaked real CLAUDE_CONFIG_DIR; re-added below as isolated value
		default:
			result = append(result, kv)
		}
	}
	if !pathHandled {
		result = append(result, "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	// Pin CLAUDE_CONFIG_DIR to the fake home so the harness update target for
	// claude-credential-guard writes to the isolated dir, not the real one.
	result = append(result, "CLAUDE_CONFIG_DIR="+filepath.Join(fakeHome, ".claude"))
	return result
}

// d15RunTFW runs the compiled trackfw binary in workDir with the isolated env.
// Returns combined stdout+stderr and the exit code.
func d15RunTFW(t *testing.T, workDir, fakeHome string, args ...string) (string, int) {
	t.Helper()
	bin := e2eBinary(t)
	binDir := filepath.Dir(bin)
	cmd := exec.Command(bin, args...)
	cmd.Dir = workDir
	cmd.Env = d15CleanEnv(fakeHome, binDir)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return buf.String(), exitErr.ExitCode()
		}
		t.Fatalf("d15RunTFW %v in %s: unexpected exec error: %v\noutput: %s",
			args, workDir, err, buf.String())
	}
	return buf.String(), 0
}

// d15Git runs a git command in dir with the isolated env.
func d15Git(t *testing.T, dir, fakeHome string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e2eCleanEnv(fakeHome)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// d15GitOutput runs a git command and returns trimmed stdout. Fatal on error.
func d15GitOutput(t *testing.T, dir, fakeHome string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e2eCleanEnv(fakeHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
	return strings.TrimSpace(string(out))
}

// d15SetupRepo creates an isolated bare origin + work tree, wires the remote,
// and sets git identity in the work tree. Returns (workDir, bareDir, fakeHome).
// No commits are made; callers add content and commit.
func d15SetupRepo(t *testing.T) (workDir, bareDir, fakeHome string) {
	t.Helper()
	fakeHome = t.TempDir()
	bareDir = t.TempDir()
	workDir = t.TempDir()

	// Create the global gitconfig that e2eCleanEnv points GIT_CONFIG_GLOBAL at.
	// Includes gc.auto=0 and maintenance.auto=false so that receive-pack's detached
	// git maintenance --auto does not race with t.TempDir's RemoveAll.
	_ = os.WriteFile(
		filepath.Join(fakeHome, ".gitconfig"),
		[]byte("[gc]\n\tauto = 0\n[maintenance]\n\tauto = false\n"),
		0600,
	)

	d15Git(t, bareDir, fakeHome, "init", "-q", "--bare", "-b", "main", ".")
	d15Git(t, workDir, fakeHome, "init", "-q", "-b", "main", ".")
	d15Git(t, workDir, fakeHome, "config", "user.email", "d15@localhost")
	d15Git(t, workDir, fakeHome, "config", "user.name", "D15 Test")
	d15Git(t, workDir, fakeHome, "config", "commit.gpgsign", "false")
	d15Git(t, workDir, fakeHome, "config", "core.longpaths", "true")
	d15Git(t, workDir, fakeHome, "remote", "add", "origin", bareDir)

	return workDir, bareDir, fakeHome
}

// d15HookRelPath returns the hook file path for cli, relative to the project root.
// Uses hookFileForCLI with an empty project root to get the bare relative form.
func d15HookRelPath(cli string) string {
	return hookFileForCLI("", cli)
}

// d15ValidateResult is the minimal subset of validator.ValidateResult needed to
// parse `trackfw validate --json` output without importing the validator package.
type d15ValidateResult struct {
	Summary struct {
		ExitCode   int `json:"exit_code"`
		Violations int `json:"violations"`
	} `json:"summary"`
	Violations []struct {
		Rule    string `json:"rule"`
		Message string `json:"message"`
	} `json:"violations"`
}

// d15ParseValidateJSON unmarshals the first JSON object found in raw. Fatal on error.
func d15ParseValidateJSON(t *testing.T, raw string) d15ValidateResult {
	t.Helper()
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		t.Fatalf("d15ParseValidateJSON: no JSON object found in output:\n%s", raw)
	}
	var result d15ValidateResult
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		t.Fatalf("d15ParseValidateJSON: unmarshal failed: %v\nraw:\n%s", err, raw)
	}
	return result
}

// TestD15_UpdatePreservesProjectGuardWithGlobalInstalled verifies that running
// `trackfw update` on a project whose origin/main contains full guard wiring, while
// the global harness guard is installed, does NOT remove the project-scope guard —
// and that `trackfw validate` subsequently returns rc=0 with zero guard_wiring_removed
// violations. The table covers Claude, Codex, Gemini, Cursor, Copilot, Kiro, Windsurf.
//
// Reconciliation: each subtest affirms D15 (ADR-2026-10-04 adendo 2026-10-24) — the
// per-CLI update path does not apply the D14 short-circuit ("global installed → skip
// project wiring") that caused guard_wiring_removed to fire after `trackfw update`
// when the global guard was present (bug reported in issue #613).
func TestD15_UpdatePreservesProjectGuardWithGlobalInstalled(t *testing.T) {
	for _, row := range d15Rows {
		row := row
		t.Run(row.cli, func(t *testing.T) {
			workDir, _, fakeHome := d15SetupRepo(t)

			// ── Step 1: init project without global guard ──────────────────────
			out, rc := d15RunTFW(t, workDir, fakeHome, "init", "--ai-tools", row.cli)
			if rc != 0 {
				t.Fatalf("trackfw init --ai-tools %s: rc=%d\n%s", row.cli, rc, out)
			}

			// Precondition: hook file created by init.
			hookRel := d15HookRelPath(row.cli)
			hookAbs := filepath.Join(workDir, hookRel)
			if _, err := os.Stat(hookAbs); err != nil {
				t.Fatalf("init: hook file %s not created: %v", hookRel, err)
			}

			// ── Step 2: commit all files + push to origin/main ─────────────────
			d15Git(t, workDir, fakeHome, "add", "-A")
			d15Git(t, workDir, fakeHome, "commit", "-q", "-m", "init "+row.cli)
			// -u sets the upstream tracking ref; creates refs/remotes/origin/main,
			// which is what deriveOriginDefaultBranch() looks up.
			d15Git(t, workDir, fakeHome, "push", "-q", "-u", "origin", "main")

			// Precondition: hook file must be tracked in origin/main.
			// If it were .gitignored, guard_wiring_removed would never fire and
			// the test would pass vacuously without exercising the D15 fix.
			lsTree := d15GitOutput(t, workDir, fakeHome,
				"ls-tree", "origin/main", "--name-only", "--", hookRel)
			if lsTree == "" {
				t.Fatalf("precondition: %s not tracked in origin/main — cannot"+
					" exercise guard_wiring_removed (check .gitignore)", hookRel)
			}

			// ── Step 3: install global harness guard ───────────────────────────
			harnessTargets := strings.Join(row.harness, ",")
			out, rc = d15RunTFW(t, workDir, fakeHome,
				"update", "harness",
				"--targets", harnessTargets,
				"--install-missing",
			)
			if rc != 0 {
				t.Fatalf("trackfw update harness --targets %s: rc=%d\n%s",
					harnessTargets, rc, out)
			}

			// ── Step 4: run project-scope update (the operation under test) ────
			out, rc = d15RunTFW(t, workDir, fakeHome, "update")
			if rc != 0 {
				t.Fatalf("trackfw update: rc=%d\n%s", rc, out)
			}

			// ── Step 5: validate — must pass with zero guard_wiring_removed ────
			out, rc = d15RunTFW(t, workDir, fakeHome, "validate", "--json")
			result := d15ParseValidateJSON(t, out)

			var guardRemovedMsgs []string
			for _, v := range result.Violations {
				if v.Rule == "guard_wiring_removed" {
					guardRemovedMsgs = append(guardRemovedMsgs, v.Message)
				}
			}
			if len(guardRemovedMsgs) > 0 {
				t.Fatalf("D15 violation for %s: guard_wiring_removed fired after"+
					" update with global guard installed — project-scope guard was removed:\n%s\nfull output:\n%s",
					row.cli, strings.Join(guardRemovedMsgs, "\n"), out)
			}
			if rc != 0 {
				t.Fatalf("validate rc=%d (non-zero) for %s after update with global guard"+
					" installed — %d violation(s):\nfull output:\n%s",
					rc, row.cli, result.Summary.Violations, out)
			}

			// ── Step 6: hook file still has project-scope credential guard ─────
			data, err := os.ReadFile(hookAbs)
			if err != nil {
				t.Fatalf("read %s: %v", hookRel, err)
			}
			if !strings.Contains(string(data), "trackfw guard credential") {
				t.Fatalf("D15 violation for %s: hook file %s lost 'trackfw guard credential'"+
					" after update with global guard installed:\n%s",
					row.cli, hookRel, string(data))
			}
		})
	}
}

// TestD15_GlobalOnlyDoesNotSatisfyProjectScope is ArmB: with the global harness
// guard installed but the project hook file rewritten WITHOUT the credential guard,
// `trackfw validate` must report a violation.
//
// Windsurf is the representative CLI (same premise applies to all CLIs with a global
// guard).
//
// Reconciliation: affirms that the D14 global short-circuit in
// validateCredentialGuardPresenceRequired is gone — global installation alone does
// not satisfy the project-scope requirement. D15 requires the credential guard in
// the project hook file regardless of global state.
func TestD15_GlobalOnlyDoesNotSatisfyProjectScope(t *testing.T) {
	const cli = "windsurf"

	workDir, _, fakeHome := d15SetupRepo(t)

	// Step 1: init project with windsurf so trackfw.yaml + hooks.json are created.
	out, rc := d15RunTFW(t, workDir, fakeHome, "init", "--ai-tools", cli)
	if rc != 0 {
		t.Fatalf("init --ai-tools windsurf: rc=%d\n%s", rc, out)
	}

	// Step 2: install global windsurf credential guard.
	out, rc = d15RunTFW(t, workDir, fakeHome,
		"update", "harness",
		"--targets", "windsurf-credential-guard",
		"--install-missing",
	)
	if rc != 0 {
		t.Fatalf("update harness windsurf-credential-guard: rc=%d\n%s", rc, out)
	}

	// Step 3: overwrite .windsurf/hooks.json WITHOUT the credential guard.
	// This simulates a project file that lost the guard (e.g. edited manually or
	// generated by an older version). A non-guard command is present so the file
	// is valid JSON and exercises credential_guard_hook_resolvable (file exists,
	// guard absent).
	windsurfDir := filepath.Join(workDir, ".windsurf")
	noGuardJSON := `{"hooks":{"pre_run_command":[{"command":"echo hello","show_output":true}],"pre_write_code":[{"command":"echo hello","show_output":true}]}}`
	if err := os.WriteFile(
		filepath.Join(windsurfDir, "hooks.json"),
		[]byte(noGuardJSON),
		0o644,
	); err != nil {
		t.Fatalf("write .windsurf/hooks.json: %v", err)
	}

	// Step 4: validate must fail with credential_guard_hook_resolvable — the project hook
	// file exists but has no credential guard entry (not just rc != 0; any unrelated
	// violation must not satisfy the D15 regression).
	out, rc = d15RunTFW(t, workDir, fakeHome, "validate", "--json")
	if rc == 0 {
		t.Fatal("D15 ArmB: validate should fail when .windsurf/hooks.json exists without " +
			"the credential guard, even though the global harness guard is installed — " +
			"global alone does not satisfy the D15 project-scope requirement")
	}
	result := d15ParseValidateJSON(t, out)
	foundRule := false
	for _, v := range result.Violations {
		if v.Rule == "credential_guard_hook_resolvable" {
			foundRule = true
			break
		}
	}
	if !foundRule {
		t.Fatalf("D15 ArmB: expected violation rule 'credential_guard_hook_resolvable' but got: %v\nfull output:\n%s",
			func() []string {
				rules := make([]string, len(result.Violations))
				for i, v := range result.Violations {
					rules[i] = v.Rule
				}
				return rules
			}(), out)
	}
}
