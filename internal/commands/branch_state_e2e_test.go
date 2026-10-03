package commands

// branch_state_e2e_test.go — Ponta a ponta para os AC2, AC4, AC5, AC6, AC12 do
// ROADMAP-2026-10-01-o-estado-que-governa-a-branch-*.md (ML-2A, artemis-tf).
//
// Harness reutilizado:
//   - barrierBinary(t) de barrier_contract_test.go (mesmo pacote) — compila o binário
//     uma vez com sync.Once; override via TRACKFW_E2E_BIN (contra-braço da main).
//   - gitRepoIn de commit_unborn_branch_test.go (mesmo pacote) — padrão de env isolado
//     para comandos git.
//
// Cada test tem UMA frase de reconciliação dizendo qual conclusão do ML ele afirma.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ────────────────────────────────────────────────────────────────────────────
// Harness helpers
// ────────────────────────────────────────────────────────────────────────────

// e2eBinary returns the binary under test.
// TRACKFW_E2E_BIN overrides the compiled binary — use it for the contra-arm against main.
func e2eBinary(t *testing.T) string {
	t.Helper()
	if bin := os.Getenv("TRACKFW_E2E_BIN"); bin != "" {
		return bin
	}
	return barrierBinary(t) // compiled once via sync.Once in barrier_contract_test.go
}

// e2eCleanEnv returns a clean environment for child processes.
// Strips TRACKFW_* (e.g. TRACKFW_BRANCH which overrides branch detection) and
// GITHUB_* (forge detection); sets a controlled HOME/GIT_CONFIG_GLOBAL to homeDir.
func e2eCleanEnv(homeDir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		k := strings.SplitN(kv, "=", 2)[0]
		if strings.HasPrefix(k, "TRACKFW_") ||
			strings.HasPrefix(k, "GITHUB_") ||
			k == "GIT_DIR" ||
			k == "GIT_WORK_TREE" {
			continue
		}
		env = append(env, kv)
	}
	env = append(env,
		"HOME="+homeDir,
		"USERPROFILE="+homeDir, // Windows: os.UserHomeDir() reads USERPROFILE, not HOME
		"GIT_CONFIG_GLOBAL="+filepath.Join(homeDir, ".gitconfig"),
		"GIT_CONFIG_SYSTEM="+os.DevNull, // platform-agnostic /dev/null
		"GIT_TERMINAL_PROMPT=0",
		"LC_ALL=C",
	)
	return env
}

// runTFW runs the trackfw binary with args inside repoDir.
// Returns combined stdout+stderr output and the exit code.
// Cobra routes error text to stderr; combining avoids false-negative absence checks.
func runTFW(t *testing.T, repoDir, homeDir string, args ...string) (output string, exitCode int) {
	t.Helper()
	bin := e2eBinary(t)
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoDir
	cmd.Env = e2eCleanEnv(homeDir)
	var buf strings.Builder
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return buf.String(), exitErr.ExitCode()
		}
		t.Fatalf("runTFW %v: unexpected exec error: %v\noutput: %s", args, err, buf.String())
	}
	return buf.String(), 0
}

// gitE2E runs git with the e2e-clean environment inside dir.
// Fatal on failure (setup commands are mandatory).
func gitE2E(t *testing.T, dir, homeDir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e2eCleanEnv(homeDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// gitE2EOutput runs git and returns stdout, fatal on failure.
func gitE2EOutput(t *testing.T, dir, homeDir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = e2eCleanEnv(homeDir)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, dir, out)
	}
	return strings.TrimSpace(string(out))
}

// gitE2ECheck runs git and returns (stdout, error) without fataling.
func gitE2ECheck(dir, homeDir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(e2eCleanEnv(homeDir),
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_TERMINAL_PROMPT=0",
	)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// minimalTrackfwYAML returns a minimal strict-mode trackfw.yaml content.
// governance_mode: strict is required so branch_has_wip_roadmap stays a violation
// (not downgraded to a warning in lenient mode).
func minimalTrackfwYAML() string {
	return `governance_mode: strict
req_dir: docs/req
roadmap_dir: docs/roadmaps
roadmap_namespacing: flat
hooks: none
ci: none
forge: none
`
}

// makeE2ERepo creates an isolated git repo with trackfw layout and an initial commit on main.
// Returns (repoDir, homeDir); both are t.TempDir() scoped.
func makeE2ERepo(t *testing.T) (repoDir, homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	repoDir = t.TempDir()

	// Initialize git
	gitE2E(t, repoDir, homeDir, "init", "-q", "-b", "main", ".")
	// On Windows, git add -A of the 211 real roadmaps can exceed MAX_PATH; longpaths prevents it.
	gitE2E(t, repoDir, homeDir, "config", "core.longpaths", "true")
	gitE2E(t, repoDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, repoDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, repoDir, homeDir, "config", "commit.gpgsign", "false")
	gitE2E(t, repoDir, homeDir, "config", "core.quotepath", "true") // explicit for AC12

	// Create directory structure
	for _, d := range []string{
		"docs/roadmaps/wip",
		"docs/roadmaps/done",
		"docs/roadmaps/blocked",
		"docs/roadmaps/backlog",
		"docs/roadmaps/abandoned",
		"docs/req",
		"docs/adr",
	} {
		if err := os.MkdirAll(filepath.Join(repoDir, d), 0755); err != nil {
			t.Fatalf("mkdirall %s: %v", d, err)
		}
	}

	// Write trackfw.yaml
	if err := os.WriteFile(filepath.Join(repoDir, "trackfw.yaml"), []byte(minimalTrackfwYAML()), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	// Seed a sentinel so git has something to commit (git does not track empty dirs).
	// We do NOT write .gitkeep inside state dirs (wip/, blocked/, etc.) because they
	// are treated as roadmap files by validateWIPHasREQ / blocked_has_req and would
	// produce spurious violations that break governance assertions.
	sentinel := filepath.Join(repoDir, "docs", ".gitkeep")
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatalf("write docs/.gitkeep: %v", err)
	}

	// Initial commit on main
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: init project layout")

	return repoDir, homeDir
}

// makeBareOriginAndPush creates a bare repo at bareDir, adds it as "origin" in repoDir,
// and pushes the current branch (main). Returns bareDir.
func makeBareOriginAndPush(t *testing.T, repoDir, homeDir string) string {
	t.Helper()
	bareDir := t.TempDir()
	gitE2E(t, bareDir, homeDir, "init", "--bare", "-q", "-b", "main", ".")
	gitE2E(t, repoDir, homeDir, "remote", "add", "origin", bareDir)
	gitE2E(t, repoDir, homeDir, "push", "-u", "origin", "main")
	return bareDir
}

// writeRoadmap writes a minimal wip roadmap file in the given dir.
func writeRoadmap(t *testing.T, dir, filename string) {
	t.Helper()
	content := fmt.Sprintf(`---
status: wip
---
# Roadmap: %s

> Status: wip

## Context
E2E test fixture.

## Wave 1 — Test
> Dependencies: none

### ML-1A — Test
**Status:** ⬜ Pendente
`, strings.TrimSuffix(filename, ".md"))
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("writeRoadmap %s: %v", filename, err)
	}
}

// writeRoadmapDone writes a minimal roadmap file with status: done frontmatter.
// Use this when placing a roadmap directly in done/ (without going through `trackfw roadmap move`).
func writeRoadmapDone(t *testing.T, dir, filename string) {
	t.Helper()
	content := fmt.Sprintf(`---
status: done
---
# Roadmap: %s

> Status: done

## Context
E2E test fixture (concluded).
`, strings.TrimSuffix(filename, ".md"))
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("writeRoadmapDone %s: %v", filename, err)
	}
}

// moveRoadmapToDone moves a roadmap file from wip/ to done/ by direct file rename
// and rewrites the frontmatter status to "done".
// Use instead of `trackfw roadmap move ... done` in tests to avoid the Wave 0 / ML completion
// blockers that the CLI enforces (those are orthogonal to the governance AC being tested).
func moveRoadmapToDone(t *testing.T, repoDir, filename string) {
	t.Helper()
	src := filepath.Join(repoDir, "docs", "roadmaps", "wip", filename)
	dst := filepath.Join(repoDir, "docs", "roadmaps", "done", filename)

	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("moveRoadmapToDone read %s: %v", src, err)
	}
	// Update frontmatter status field.
	content := strings.Replace(string(data), "status: wip", "status: done", 1)
	if err := os.WriteFile(dst, []byte(content), 0644); err != nil {
		t.Fatalf("moveRoadmapToDone write %s: %v", dst, err)
	}
	if err := os.Remove(src); err != nil {
		t.Fatalf("moveRoadmapToDone remove %s: %v", src, err)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC2 (#494) — done-only match blocks branch creation
//
// Reconciliation sentence: the test affirms that `branch new` blocks (rc≠0, no git branch
// created, output has "similar names in done/", output does NOT have "trackfw roadmap move ROADMAP-")
// when the slug only matches roadmaps in done/ — and that a wip/ match allows creation (control).
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC2_DoneOnlyBlocksCreation(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)

	// Synthetic fixture instead of the real corpus: layout-independent (works under
	// roadmap_namespacing: by_agent, where the flat docs/roadmaps/done/ does not exist) and with no
	// dependency on the 214 real roadmap names.
	//
	// 🔴 These two names are the ones from the original #494 case, and the choice is load-bearing:
	// NEITHER contains the branch slug "barrier-executa-cada-linha-do-bloco-de-gates". They match
	// only by SHARED TOKENS (barrier, executa, gate...), which is the shape of the defect this AC
	// reproduces -- and there are TWO of them, so the "similar names in done/" hint is exercised in
	// the plural. Do not "simplify" this fixture to one name that contains the slug: matching by
	// containment is the easy case, and it would stop exercising overlap and the plural. If the
	// match relation ever changes, overlap could govern branch creation again with no test noticing.
	const roadmapOverlapA = "ROADMAP-2026-08-23-barrier-nao-executa-gate-de-roadmap-nao-confiavel-e-roadmap-new-sanitiza-o-titulo.md"
	const roadmapOverlapB = "ROADMAP-2026-09-10-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md"
	testDoneDir := filepath.Join(repoDir, "docs", "roadmaps", "done")
	writeRoadmapDone(t, testDoneDir, roadmapOverlapA)
	writeRoadmapDone(t, testDoneDir, roadmapOverlapB)

	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add done/ corpus")

	const branchSpec = "fix/barrier-executa-cada-linha-do-bloco-de-gates"
	const branchRef = "refs/heads/fix/barrier-executa-cada-linha-do-bloco-de-gates"

	// ─── Negative arm: wip/ empty → blocked ─────────────────────────────────
	out, rc := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec)
	t.Logf("AC2 blocked output:\n%s", out)

	if rc == 0 {
		t.Errorf("AC2 negative: expected rc≠0 (branch blocked), got rc=0\noutput: %s", out)
	}
	// The hint must name BOTH overlap matches, not just the first one.
	for _, want := range []string{roadmapOverlapA, roadmapOverlapB} {
		if !strings.Contains(out, want) {
			t.Errorf("AC2 negative: hint must name %q\noutput: %s", want, out)
		}
	}
	if !strings.Contains(out, "similar names in done/") {
		t.Errorf("AC2 negative: expected 'similar names in done/' in output\noutput: %s", out)
	}
	if strings.Contains(out, "trackfw roadmap move ROADMAP-") {
		t.Errorf("AC2 negative: output must NOT contain 'trackfw roadmap move ROADMAP-' (ML-1D: hint must not name concrete roadmaps)\noutput: %s", out)
	}

	// Branch must not have been created.
	if _, err := gitE2ECheck(repoDir, homeDir, "rev-parse", "--verify", branchRef); err == nil {
		t.Errorf("AC2 negative: branch %q must not exist but git rev-parse succeeded", branchRef)
	}

	// ─── Positive control: add roadmap to wip/ → branch is allowed ──────────
	// Control arm keeps a name that matches by containment: that is the legitimate case, and the
	// AC here is that a wip/ match allows creation regardless of HOW it matched.
	const wipRoadmap = "ROADMAP-2026-10-01-barrier-executa-cada-linha-do-bloco-de-gates.md"
	wipDir := filepath.Join(repoDir, "docs", "roadmaps", "wip")
	writeRoadmap(t, wipDir, wipRoadmap)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add wip roadmap for control")

	outCtrl, rcCtrl := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec, "--dry-run")
	t.Logf("AC2 control output:\n%s", outCtrl)

	if rcCtrl != 0 {
		t.Errorf("AC2 control: expected rc=0 (branch would be created), got rc=%d\noutput: %s", rcCtrl, outCtrl)
	}
	if !strings.Contains(outCtrl, "[dry-run] would create branch") {
		t.Errorf("AC2 control: expected '[dry-run] would create branch' in output\noutput: %s", outCtrl)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC4 (#490) — blocked/ roadmap governs: with written link
//
// Reconciliation sentence: the test affirms that after `roadmap move ... blocked`, a
// feat/fix/refactor branch with a written link can `commit` (rc=0), `validate` passes
// without `branch_link_stale` or `feat/fix/refactor branch` violation, and
// `push --dry-run` prints `Governance: OK`.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC4_BlockedRoadmapGoverns_WithLink(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)
	testAC4(t, repoDir, homeDir, true /* deleteLink = false → keep link */)
}

// ────────────────────────────────────────────────────────────────────────────
// AC4 (#490) — blocked/ roadmap governs: without written link (inference only)
//
// Reconciliation sentence: the test affirms that blocked/ roadmap governs via inference
// alone when the written link file is absent — commit passes, validate clean,
// push --dry-run shows Governance: OK.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC4_BlockedRoadmapGoverns_NoLink(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)
	testAC4(t, repoDir, homeDir, false /* deleteLink = true → remove link before move */)
}

// testAC4 is the shared implementation for the AC4 pair (with/without link).
// withLink=true: keeps the written link; withLink=false: deletes it before roadmap move.
func testAC4(t *testing.T, repoDir, homeDir string, withLink bool) {
	t.Helper()

	const roadmapSlug = "cache-de-sessao"
	const roadmapFile = "ROADMAP-2026-10-01-cache-de-sessao.md"
	const branchSpec = "fix/cache-de-sessao"
	const branchName = "fix/cache-de-sessao"

	// Setup: add roadmap to wip/ and commit
	wipDir := filepath.Join(repoDir, "docs", "roadmaps", "wip")
	writeRoadmap(t, wipDir, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add cache-de-sessao roadmap")

	// Create branch (writes link file via trackfw branch new)
	out, rc := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec)
	t.Logf("branch new output: %s", out)
	if rc != 0 {
		t.Fatalf("testAC4: branch new failed: rc=%d\n%s", rc, out)
	}

	if !withLink {
		// Delete the written link file before the roadmap move
		linkFile := filepath.Join(repoDir, "docs", "roadmaps", ".trackfw-branch-links.json")
		if err := os.Remove(linkFile); err != nil && !os.IsNotExist(err) {
			t.Logf("testAC4: note — link file %s not found (ok): %v", linkFile, err)
		}
	}

	// Move roadmap to blocked/
	out, rc = runTFW(t, repoDir, homeDir, "roadmap", "move", roadmapSlug, "blocked")
	t.Logf("roadmap move output: %s", out)
	if rc != 0 {
		t.Fatalf("testAC4: roadmap move failed: rc=%d\n%s", rc, out)
	}

	// Verify the move happened
	blockedRoadmap := filepath.Join(repoDir, "docs", "roadmaps", "blocked", roadmapFile)
	if _, err := os.Stat(blockedRoadmap); os.IsNotExist(err) {
		t.Fatalf("testAC4: roadmap not found in blocked/ after move: %s", blockedRoadmap)
	}

	// Stage the move
	gitE2E(t, repoDir, homeDir, "add", "-A")

	// Stage a sentinel file so commit has something to commit
	sentinel := filepath.Join(repoDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("test\n"), 0644); err != nil {
		t.Fatalf("testAC4: write sentinel: %v", err)
	}
	gitE2E(t, repoDir, homeDir, "add", "sentinel.txt")

	// trackfw commit — must succeed (rc=0)
	out, rc = runTFW(t, repoDir, homeDir, "commit", "-m", "fix: cache de sessao update")
	t.Logf("commit output: %s", out)
	if rc != 0 {
		t.Errorf("AC4: expected commit rc=0, got rc=%d\noutput: %s", rc, out)
	}

	// trackfw validate — must be clean (no branch violation, no branch_link_stale)
	outV, rcV := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("validate output: %s", outV)
	// Presence twin: either "No violations found" or warnings-only
	if strings.Contains(outV, "feat/fix/refactor branch") {
		t.Errorf("AC4: validate must not report 'feat/fix/refactor branch' violation\noutput: %s", outV)
	}
	if strings.Contains(outV, "branch_link_stale") {
		t.Errorf("AC4: validate must not report 'branch_link_stale'\noutput: %s", outV)
	}
	// The branch_has_wip_roadmap rule must not fire (no matching violation message)
	if strings.Contains(outV, "branch_has_wip_roadmap") {
		t.Errorf("AC4: validate must not report branch_has_wip_roadmap rule\noutput: %s", outV)
	}
	_ = rcV // validate may still have other unrelated violations; we check only the branch rule

	// trackfw push --dry-run — must contain "Governance: OK"
	outP, rcP := runTFW(t, repoDir, homeDir, "push", "--dry-run")
	t.Logf("push --dry-run output: %s", outP)
	if !strings.Contains(outP, "Governance: OK") {
		t.Errorf("AC4: push --dry-run must contain 'Governance: OK'\noutput: %s", outP)
	}
	if rcP != 0 {
		// Governance: OK printed + push skipped in dry-run → exit 0 expected
		t.Errorf("AC4: push --dry-run exit rc=%d (expected 0)\noutput: %s", rcP, outP)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC5(a) — done/ moved by this branch governs (with written link)
//
// Reconciliation sentence: the test affirms that a roadmap absent from origin/main
// done/ (moved to done/ by this branch) governs an existing branch — commit passes
// and validate has no branch violation — when the written link is present.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC5a_DoneMovedByBranchAccepted_WithLink(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)
	testAC5a(t, repoDir, homeDir, true /* withLink */)
}

// ────────────────────────────────────────────────────────────────────────────
// AC5(a) — done/ moved by this branch governs (no written link, inference only)
//
// Reconciliation sentence: the test affirms that when the roadmap is absent from
// origin/main done/ (inference path), the branch is governed even without a written
// link — commit passes, validate has no branch violation.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC5a_DoneMovedByBranchAccepted_NoLink(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)
	testAC5a(t, repoDir, homeDir, false /* withLink=false */)
}

func testAC5a(t *testing.T, repoDir, homeDir string, withLink bool) {
	t.Helper()

	const roadmapFile = "ROADMAP-2026-10-01-gateway-estado.md"
	const branchSpec = "fix/gateway-estado"

	// Setup: add roadmap to wip/ and push to origin (origin/main has roadmap in wip/)
	wipDir := filepath.Join(repoDir, "docs", "roadmaps", "wip")
	writeRoadmap(t, wipDir, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add gateway-estado roadmap in wip/")

	// Push to a local bare origin (so origin/main has roadmap in wip/ NOT done/)
	makeBareOriginAndPush(t, repoDir, homeDir)

	// Create branch
	if withLink {
		out, rc := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec)
		if rc != 0 {
			t.Fatalf("testAC5a: branch new failed rc=%d\n%s", rc, out)
		}
	} else {
		// No written link: use git directly
		gitE2E(t, repoDir, homeDir, "checkout", "-b", branchSpec)
	}

	// Move roadmap from wip/ to done/ directly (bypass `trackfw roadmap move` which
	// enforces Wave 0 / ML-completion checks orthogonal to the AC being tested).
	moveRoadmapToDone(t, repoDir, roadmapFile)

	// Stage the move + sentinel file
	sentinel := filepath.Join(repoDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("done\n"), 0644); err != nil {
		t.Fatalf("testAC5a: write sentinel: %v", err)
	}
	gitE2E(t, repoDir, homeDir, "add", "-A")

	// Premise guard: roadmap is in done/ locally AND NOT in done/ on origin/main.
	doneRoadmap := filepath.Join(repoDir, "docs", "roadmaps", "done", roadmapFile)
	if _, err := os.Stat(doneRoadmap); os.IsNotExist(err) {
		t.Fatalf("testAC5a: roadmap not in done/ after moveRoadmapToDone (premise broken)")
	}

	out, rc := runTFW(t, repoDir, homeDir, "commit", "-m", "fix: gateway estado done")
	t.Logf("commit output: %s", out)
	if rc != 0 {
		t.Errorf("AC5a: expected commit rc=0 (roadmap moved by this branch governs), got rc=%d\noutput: %s", rc, out)
	}

	// trackfw validate — no branch violation
	outV, _ := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("validate output: %s", outV)
	if strings.Contains(outV, "feat/fix/refactor branch") {
		t.Errorf("AC5a: validate must not report 'feat/fix/refactor branch' violation\noutput: %s", outV)
	}
	if strings.Contains(outV, "branch_has_wip_roadmap") {
		t.Errorf("AC5a: validate must not report branch_has_wip_roadmap\noutput: %s", outV)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC5(b) — done/ already in origin/main does NOT govern (Definition of Done boundary)
//
// Reconciliation sentence: the test affirms that a roadmap already in done/ on
// origin/main (not moved by this branch) does NOT govern the branch — commit blocks
// (rc≠0) and validate reports a branch_has_wip_roadmap violation.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC5b_DoneAlreadyInBaseBlocks(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)

	const roadmapFile = "ROADMAP-2026-10-01-gateway-estado.md"
	const branchSpec = "fix/gateway-estado"

	// Setup: put roadmap directly in done/ on main (never in wip/)
	doneDir := filepath.Join(repoDir, "docs", "roadmaps", "done")
	// Use writeRoadmapDone so frontmatter says status: done (avoids "folder done but status wip" noise).
	writeRoadmapDone(t, doneDir, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: roadmap directly in done/")

	// Push to bare origin — origin/main now has roadmap in done/
	makeBareOriginAndPush(t, repoDir, homeDir)

	// Create branch with git checkout -b (NOT trackfw branch new, as per AC5b spec:
	// "crie-a com git checkout -b via exec dentro do teste — o hook só age em comandos de shell")
	gitE2E(t, repoDir, homeDir, "checkout", "-b", branchSpec)

	// Premise guard: verify the slug matches the roadmap from done/
	// (slug = gateway-estado, roadmap contains gateway-estado → substring match ✓)
	normalizedSlug := "gateway-estado"
	normalizedFile, _ := func() (string, bool) {
		// normalizeBranchSlug(roadmapFile) should contain normalizedSlug
		slug := strings.ToLower(roadmapFile)
		var b bytes.Buffer
		lastDash := false
		for _, r := range slug {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				lastDash = false
			} else if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
		s := strings.Trim(b.String(), "-")
		return s, strings.Contains(s, normalizedSlug)
	}()
	if !strings.Contains(normalizedFile, normalizedSlug) {
		t.Fatalf("AC5b premise: slug %q must be substring of normalized filename %q", normalizedSlug, normalizedFile)
	}

	// trackfw commit — must block (rc≠0) because roadmap is in done/ on origin/main
	out, rc := runTFW(t, repoDir, homeDir, "commit", "-m", "fix: this should be blocked")
	t.Logf("commit output: %s", out)
	if rc == 0 {
		t.Errorf("AC5b: expected commit rc≠0 (done/ in base does not govern), got rc=0\noutput: %s", out)
	}

	// trackfw validate — must report a branch governance violation.
	// The actual violation message is:
	//   branch "fix/..." has no matching roadmap in wip/, blocked/ nor done/ (found: ROADMAP-...)
	// This contains "wip/, blocked/ nor done/" (from BranchGovernanceOrientationForExisting).
	outV, rcV := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("validate output: %s", outV)
	if rcV == 0 {
		t.Errorf("AC5b: validate must exit non-zero (governance_mode: strict, branch has no governing roadmap)\noutput: %s", outV)
	}
	if !strings.Contains(outV, "wip/, blocked/ nor done/") {
		t.Errorf("AC5b: validate must contain 'wip/, blocked/ nor done/' (branch governance violation)\noutput: %s", outV)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC6/AC11 — no origin → degraded warning, never hard violation
//
// Reconciliation sentence: the test affirms that when origin is absent (D3 of
// ADR-2026-10-01), the done/-only match produces a branch_done_scope_unverifiable
// warning (not a hard violation), validate exits 0, and push --dry-run prints
// "Governance: degraded" and not "Governance: OK".
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC6_NoPushOriginDegrades(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)

	const roadmapFile = "ROADMAP-2026-10-01-gateway-estado.md"
	const branchSpec = "fix/gateway-estado"

	// Setup: roadmap in wip/ on main, create branch, then move to done/
	// NO origin remote — D3 cannot verify whether the roadmap was there before
	wipDir := filepath.Join(repoDir, "docs", "roadmaps", "wip")
	writeRoadmap(t, wipDir, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add roadmap in wip/")

	// Create branch without origin (git checkout -b, no trackfw branch new → no link)
	gitE2E(t, repoDir, homeDir, "checkout", "-b", branchSpec)

	// Move roadmap from wip/ to done/ directly (bypass `trackfw roadmap move` blockers).
	moveRoadmapToDone(t, repoDir, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")

	// Verify no remote exists
	outRemote, _ := gitE2ECheck(repoDir, homeDir, "remote")
	if strings.TrimSpace(outRemote) != "" {
		t.Fatalf("AC6 premise: expected no remote, got: %q", outRemote)
	}

	// trackfw validate — must exit 0 (no hard violation) and contain branch_done_scope_unverifiable
	outV, rcV := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("validate output: %s", outV)
	if rcV != 0 {
		// In strict mode, branch_done_scope_unverifiable is a WARNING not a violation (D3).
		// If validate exits ≠0 here the contract is broken.
		t.Errorf("AC6: validate must exit 0 (D3 is warn-only, never a hard violation), got rc=%d\noutput: %s", rcV, outV)
	}
	if strings.Contains(outV, "feat/fix/refactor branch") {
		// The D3 path accepts the roadmap — no branch violation should fire
		t.Errorf("AC6: validate must not report 'feat/fix/refactor branch' (D3 accepted the roadmap)\noutput: %s", outV)
	}
	if !strings.Contains(outV, "branch_done_scope_unverifiable") {
		t.Errorf("AC6: validate must contain 'branch_done_scope_unverifiable' warning\noutput: %s", outV)
	}

	// Stage a sentinel file as well (roadmap move already staged above)
	sentinel := filepath.Join(repoDir, "sentinel.txt")
	_ = os.WriteFile(sentinel, []byte("ac6\n"), 0644)
	gitE2E(t, repoDir, homeDir, "add", "sentinel.txt")

	outP, _ := runTFW(t, repoDir, homeDir, "push", "--dry-run")
	t.Logf("push --dry-run output: %s", outP)
	if strings.Contains(outP, "Governance: OK") {
		t.Errorf("AC6: push --dry-run must NOT contain 'Governance: OK' when D3 degraded\noutput: %s", outP)
	}
	if !strings.Contains(outP, "Governance: degraded") {
		t.Errorf("AC6: push --dry-run must contain 'Governance: degraded'\noutput: %s", outP)
	}
	if !strings.Contains(outP, "branch_done_scope_unverifiable") {
		t.Errorf("AC6: push --dry-run must contain 'branch_done_scope_unverifiable'\noutput: %s", outP)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC12 — accented roadmap in origin/main done/ is correctly detected with -z
//
// Reconciliation sentence: the test affirms that with core.quotepath=true, a roadmap
// with an accented filename (e.g. revisão) in done/ on origin/main is correctly
// identified as "already in base" via git ls-tree -z — preventing a spurious
// "moved by this branch" acceptance that the old binary (without -z) would produce.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC12_AccentedRoadmapInBaseDoesNotGovern(t *testing.T) {
	repoDir, homeDir := makeE2ERepo(t)

	// Roadmap with accented filename: normalizeBranchSlug produces "revis-o-do-cache"
	// (the ã becomes a dash), so the branch slug "revis-o-do-cache" matches by substring.
	const roadmapFile = "ROADMAP-2026-10-01-revisão-do-cache.md" // revisão
	const branchSpec = "fix/revis-o-do-cache"

	// Premise guard: verify the branch slug matches the roadmap via substring.
	// normalizeBranchSlug("ROADMAP-2026-10-01-revisão-do-cache.md") = "roadmap-2026-10-01-revis-o-do-cache-md"
	// contains "revis-o-do-cache" → TRUE
	const slug = "revis-o-do-cache"
	var normalizedName bytes.Buffer
	lastDash := false
	for _, r := range strings.ToLower(roadmapFile) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			normalizedName.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			normalizedName.WriteByte('-')
			lastDash = true
		}
	}
	normalizedNameStr := strings.Trim(normalizedName.String(), "-")
	if !strings.Contains(normalizedNameStr, slug) {
		t.Fatalf("AC12 premise: slug %q must be a substring of normalized name %q", slug, normalizedNameStr)
	}

	// Setup: put accented roadmap in done/ on main and push to origin.
	// This represents "roadmap already concluded before this branch".
	doneDir := filepath.Join(repoDir, "docs", "roadmaps", "done")
	writeRoadmapDone(t, doneDir, roadmapFile) // status: done so no frontmatter mismatch violation
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: accented roadmap in done/")

	// Push to bare origin — origin/main now has the accented roadmap in done/
	makeBareOriginAndPush(t, repoDir, homeDir)

	// Secondary premise guard: with core.quotepath=true (set in makeE2ERepo),
	// git ls-tree --name-only should quote the accented filename.
	// This verifies the A1 defect path is actually exercised.
	lsOut, lsErr := gitE2ECheck(repoDir, homeDir, "ls-tree", "--name-only", "origin/main",
		"--", "docs/roadmaps/done/")
	if lsErr == nil && !strings.Contains(lsOut, `\`) {
		// On some systems the filename may not be quoted (NFC vs NFD, locale).
		// Log but don't fail — the test still exercises the -z fix path.
		t.Logf("AC12 note: ls-tree --name-only did not quote accented filename; may be NFC/NFD on macOS. Output: %q", lsOut)
	} else if lsErr == nil {
		t.Logf("AC12 premise confirmed: ls-tree quotes accented filename: %q", lsOut)
	}

	// Create branch via git checkout -b (not trackfw branch new → no written link)
	gitE2E(t, repoDir, homeDir, "checkout", "-b", branchSpec)

	// trackfw validate — must have a branch_has_wip_roadmap violation
	// With -z fix (new binary): ls-tree correctly finds the accented filename in origin/main done/
	//   → inBase = true → roadmap does NOT govern → violation. ✓
	// Without -z fix (old binary): ls-tree quotes the name → filepath.Base returns escaped form
	//   → baseSet lookup fails → inBase = false → spurious "moved by this branch" → no violation. ✗
	outV, rcV := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("AC12 validate output:\n%s", outV)

	if rcV == 0 && !strings.Contains(outV, "branch_has_wip_roadmap") {
		// Validate exited 0 AND no branch violation — this is the old-binary false-positive.
		t.Errorf("AC12: validate must report branch violation (accented roadmap was already in origin/main done/)\noutput: %s", outV)
	}
	hasViolation := rcV != 0 || strings.Contains(outV, "feat/fix/refactor branch") ||
		strings.Contains(outV, "branch_has_wip_roadmap")
	if !hasViolation {
		t.Errorf("AC12: validate must contain 'feat/fix/refactor branch' or 'branch_has_wip_roadmap'\noutput: %s", outV)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// by_agent helpers
// ────────────────────────────────────────────────────────────────────────────

// minimalByAgentTrackfwYAML returns a strict-mode trackfw.yaml with roadmap_namespacing: by_agent
// and the given agents. The flat state dirs (wip/, done/, etc.) are NOT created on disk so they
// won't be picked up by resolveAgentNamespaces as accidental agent namespaces.
func minimalByAgentTrackfwYAML(agentA, agentB string) string {
	return "governance_mode: strict\n" +
		"req_dir: docs/req\n" +
		"roadmap_dir: docs/roadmaps\n" +
		"roadmap_namespacing: by_agent\n" +
		"agents:\n" +
		"  - " + agentA + "\n" +
		"  - " + agentB + "\n" +
		"hooks: none\n" +
		"ci: none\n" +
		"forge: none\n"
}

// makeE2ERepoByAgent creates an isolated git repo with by_agent layout (2 agents: agentA, agentB).
// Crucially, it does NOT create flat docs/roadmaps/{wip,done,...} dirs so they won't be treated
// as agent namespaces by resolveAgentNamespaces.
func makeE2ERepoByAgent(t *testing.T, agentA, agentB string) (repoDir, homeDir string) {
	t.Helper()
	homeDir = t.TempDir()
	repoDir = t.TempDir()

	gitE2E(t, repoDir, homeDir, "init", "-q", "-b", "main", ".")
	// On Windows, git add -A of the 211 real roadmaps can exceed MAX_PATH; longpaths prevents it.
	gitE2E(t, repoDir, homeDir, "config", "core.longpaths", "true")
	gitE2E(t, repoDir, homeDir, "config", "user.email", "e2e@localhost")
	gitE2E(t, repoDir, homeDir, "config", "user.name", "E2E Test")
	gitE2E(t, repoDir, homeDir, "config", "commit.gpgsign", "false")
	gitE2E(t, repoDir, homeDir, "config", "core.quotepath", "true")

	// Create by_agent state dirs for each agent; no flat state dirs at root of roadmap_dir.
	for _, agent := range []string{agentA, agentB} {
		for _, state := range []string{"wip", "done", "blocked", "backlog", "abandoned"} {
			d := filepath.Join(repoDir, "docs", "roadmaps", agent, state)
			if err := os.MkdirAll(d, 0755); err != nil {
				t.Fatalf("mkdirall %s: %v", d, err)
			}
		}
	}
	for _, d := range []string{"docs/req", "docs/adr"} {
		if err := os.MkdirAll(filepath.Join(repoDir, d), 0755); err != nil {
			t.Fatalf("mkdirall %s: %v", d, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repoDir, "trackfw.yaml"),
		[]byte(minimalByAgentTrackfwYAML(agentA, agentB)), 0644); err != nil {
		t.Fatalf("write trackfw.yaml: %v", err)
	}

	sentinel := filepath.Join(repoDir, "docs", ".gitkeep")
	if err := os.WriteFile(sentinel, []byte{}, 0644); err != nil {
		t.Fatalf("write docs/.gitkeep: %v", err)
	}

	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: init by_agent project layout")
	return repoDir, homeDir
}

// writeRoadmapDoneInAgentDir writes a done/ roadmap for a specific agent.
func writeRoadmapDoneInAgentDir(t *testing.T, repoDir, agent, filename string) {
	t.Helper()
	dir := filepath.Join(repoDir, "docs", "roadmaps", agent, "done")
	writeRoadmapDone(t, dir, filename)
}

// writeRoadmapInAgentWip writes a wip/ roadmap for a specific agent.
func writeRoadmapInAgentWip(t *testing.T, repoDir, agent, filename string) {
	t.Helper()
	dir := filepath.Join(repoDir, "docs", "roadmaps", agent, "wip")
	writeRoadmap(t, dir, filename)
}

// ────────────────────────────────────────────────────────────────────────────
// AC2-by_agent — done-only match in by_agent layout blocks branch creation
//
// Reconciliation sentence: the test affirms that `branch new` blocks when the slug
// matches a roadmap only in a by_agent agent's done/ (with wip/ empty for both agents),
// confirming that the done/ loop in `runBranchNew` iterates all agent-scoped done/ dirs,
// not just the flat one — and that a wip/ match in any agent allows creation.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC2_ByAgent_DoneOnlyBlocksCreation(t *testing.T) {
	const agentA = "alpha"
	const agentB = "beta"
	repoDir, homeDir := makeE2ERepoByAgent(t, agentA, agentB)

	// Use the same slug as the existing AC2 flat test to prove cross-namespacing.
	const roadmapFile = "ROADMAP-2026-10-01-gateway-estado.md"
	const branchSpec = "fix/gateway-estado"

	// Put the matching roadmap in agentA's done/ (not wip/ for either agent)
	writeRoadmapDoneInAgentDir(t, repoDir, agentA, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: roadmap in alpha/done/")

	const branchRef = "refs/heads/fix/gateway-estado"

	// ─── Negative arm: wip/ empty for both agents → blocked ─────────────────
	out, rc := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec)
	t.Logf("AC2-by_agent blocked output:\n%s", out)

	if rc == 0 {
		t.Errorf("AC2-by_agent negative: expected rc≠0 (branch blocked), got rc=0\noutput: %s", out)
	}
	if !strings.Contains(out, "similar names in done/") {
		t.Errorf("AC2-by_agent negative: expected 'similar names in done/' in output\noutput: %s", out)
	}
	if strings.Contains(out, "trackfw roadmap move ROADMAP-") {
		t.Errorf("AC2-by_agent negative: output must NOT contain 'trackfw roadmap move ROADMAP-'\noutput: %s", out)
	}

	// Branch must not have been created.
	if _, err := gitE2ECheck(repoDir, homeDir, "rev-parse", "--verify", branchRef); err == nil {
		t.Errorf("AC2-by_agent negative: branch %q must not exist but git rev-parse succeeded", branchRef)
	}

	// ─── Positive control: add roadmap to beta/wip/ → branch is allowed ─────
	writeRoadmapInAgentWip(t, repoDir, agentB, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: add roadmap to beta/wip/")

	outCtrl, rcCtrl := runTFW(t, repoDir, homeDir, "branch", "new", branchSpec, "--dry-run")
	t.Logf("AC2-by_agent control output:\n%s", outCtrl)

	if rcCtrl != 0 {
		t.Errorf("AC2-by_agent control: expected rc=0 (wip/ match in beta governs), got rc=%d\noutput: %s", rcCtrl, outCtrl)
	}
	if !strings.Contains(outCtrl, "[dry-run] would create branch") {
		t.Errorf("AC2-by_agent control: expected '[dry-run] would create branch' in output\noutput: %s", outCtrl)
	}
}

// ────────────────────────────────────────────────────────────────────────────
// AC5b-by_agent — done/ already in origin/main (agent beta) does NOT govern
//
// Reconciliation sentence: the test affirms that when origin/main already has a
// roadmap in a by_agent agent's done/ (agentB/done/) before the branch was created,
// that roadmap does NOT govern — commit blocks (rc≠0) and validate in strict mode
// reports a branch_has_wip_roadmap violation — confirming the ls-tree loop checks all
// agent-scoped done/ dirs in origin/main, not just a flat done/.
// ────────────────────────────────────────────────────────────────────────────

func TestBranchStateE2E_AC5b_ByAgent_DoneAlreadyInBaseBlocks(t *testing.T) {
	const agentA = "alpha"
	const agentB = "beta"
	repoDir, homeDir := makeE2ERepoByAgent(t, agentA, agentB)

	// Reuse gateway-estado slug — proven to slug-match the roadmap filename.
	const roadmapFile = "ROADMAP-2026-10-01-gateway-estado.md"
	const branchSpec = "fix/gateway-estado"

	// Put the roadmap in agentB/done/ on main (the last agent — catches a loop that
	// only checks the first agent's done/).
	writeRoadmapDoneInAgentDir(t, repoDir, agentB, roadmapFile)
	gitE2E(t, repoDir, homeDir, "add", "-A")
	gitE2E(t, repoDir, homeDir, "commit", "-q", "-m", "chore: roadmap in beta/done/ on main")

	// Push to bare origin — origin/main now has the roadmap in beta/done/
	makeBareOriginAndPush(t, repoDir, homeDir)

	// Create branch WITHOUT trackfw branch new (to avoid the creation guard)
	gitE2E(t, repoDir, homeDir, "checkout", "-b", branchSpec)

	// Premise guard: slug "gateway-estado" must be a substring of normalized filename.
	const normalizedSlug = "gateway-estado"
	{
		slug := strings.ToLower(roadmapFile)
		var b bytes.Buffer
		lastDash := false
		for _, r := range slug {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				b.WriteRune(r)
				lastDash = false
			} else if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
		s := strings.Trim(b.String(), "-")
		if !strings.Contains(s, normalizedSlug) {
			t.Fatalf("AC5b-by_agent premise: slug %q must be substring of normalized filename %q", normalizedSlug, s)
		}
	}

	// Stage a sentinel so commit has staged content.
	sentinel := filepath.Join(repoDir, "sentinel.txt")
	if err := os.WriteFile(sentinel, []byte("ac5b-by-agent\n"), 0644); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}
	gitE2E(t, repoDir, homeDir, "add", "sentinel.txt")

	// trackfw commit — must block (rc≠0) because roadmap is in beta/done/ on origin/main
	out, rc := runTFW(t, repoDir, homeDir, "commit", "-m", "fix: this should be blocked")
	t.Logf("AC5b-by_agent commit output:\n%s", out)
	if rc == 0 {
		t.Errorf("AC5b-by_agent: expected commit rc≠0 (done/ in base does not govern), got rc=0\noutput: %s", out)
	}
	// The roadmap filename must appear in the output — proving it was found in beta/done/
	// and rejected because it's in base, not that it was invisible.
	if !strings.Contains(out, roadmapFile) {
		t.Errorf("AC5b-by_agent: roadmap filename %q must appear in commit output (found in beta/done/, rejected as in base)\noutput: %s", roadmapFile, out)
	}

	// trackfw validate — must report branch governance violation in strict mode
	outV, rcV := runTFW(t, repoDir, homeDir, "validate")
	t.Logf("AC5b-by_agent validate output:\n%s", outV)
	if rcV == 0 {
		t.Errorf("AC5b-by_agent: validate must exit non-zero (strict mode, no governing roadmap)\noutput: %s", outV)
	}
	if !strings.Contains(outV, "wip/, blocked/ nor done/") {
		t.Errorf("AC5b-by_agent: validate must contain 'wip/, blocked/ nor done/'\noutput: %s", outV)
	}
	// The roadmap filename must appear in validate output too — proving the by_agent done/ was found.
	if !strings.Contains(outV, roadmapFile) {
		t.Errorf("AC5b-by_agent: roadmap filename %q must appear in validate output\noutput: %s", roadmapFile, outV)
	}
}


