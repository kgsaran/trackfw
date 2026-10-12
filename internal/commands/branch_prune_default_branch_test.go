package commands

// branch_prune_default_branch_test.go — tests for dynamic default-branch resolution (ML-1A,
// ROADMAP-2026-10-10, ADR-2026-10-12 A7). Each function carries its reconciliation sentence.
//
// TestBranchPrune_MasterRepo_EndToEnd is the end-to-end criterion of ML-1A: "a test with a
// master repo passes; without the fix it fails". It exercises runBranchPrune with a real git
// repository whose default branch is "master". Without the A7 fix, runBranchPrune would refuse
// with "origin/main not found — … Refusing".
//
// The four stub tests below exercise resolveDefaultBranch / evaluateBranchWithForge directly.
//
// Measured bite outcomes (each bite applied to branch_prune_forge.go, reversed before next):
//
//	M1 — resolveDefaultBranch returns (branchPruneDefaultLocalName, branchPruneDefaultRemoteRef)
//	     unconditionally (first line of function body, discards forge and symref paths):
//	       TestBranchPrune_MasterRepo_EndToEnd              FAIL  "origin/main not found — Refusing"
//	       TestDefaultBranch_MasterRepo_NoForge             FAIL  localName "main" ≠ "master"
//	       TestDefaultBranch_ForgeReturnsTrunk_FilterUsesIt FAIL  localName "main" ≠ "trunk"
//	       TestDefaultBranch_ForgeVsSymref_ForgePrevails    FAIL  localName "main" ≠ "trunk"
//	       TestDefaultBranch_NeitherAvailable_FallsBackToMain PASS (fallback is unchanged)
//
//	M2 — pr.BaseRefName filter uses literal "main" instead of defaultLocalName:
//	       TestDefaultBranch_ForgeReturnsTrunk_FilterUsesIt FAIL  decision falls to content heuristic
//	       all others                                        PASS
//
//	M3 — symref consulted before forge in resolveDefaultBranch:
//	       TestDefaultBranch_ForgeVsSymref_ForgePrevails    FAIL  "main" from symref wins over "trunk"
//	       all others                                        PASS

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestBranchPrune_MasterRepo_EndToEnd asserts: on a real git repository whose default branch is
// "master" (symref refs/remotes/origin/HEAD → origin/master, ghExec nil), runBranchPrune:
// (a) does not refuse with "not found — … Refusing",
// (b) prints a header naming origin/master,
// (c) reports master as keep / default branch,
// (d) reports feat/integrada as delete (no own work relative to origin/master), and
// (e) reports feat/aberta as keep (pending work).
//
// Reconciliation: resolveDefaultBranch picks "master" from the symref when ghExec is nil; the
// resolved remoteRef "origin/master" reaches both the rev-parse gate and the content heuristic.
// Without the A7 fix (hardcoded "origin/main"), runBranchPrune refuses at the rev-parse gate,
// and assertion (a) fails.
func TestBranchPrune_MasterRepo_EndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	work := t.TempDir()
	bareDir := filepath.Join(work, "origin.git")
	cloneDir := filepath.Join(work, "clone")

	cfgPath := filepath.Join(work, "empty-gitconfig")
	if err := os.WriteFile(cfgPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	// Isolated environment: no system/global config, no prompts, deterministic author/committer.
	baseEnv := append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+cfgPath,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_AUTHOR_NAME=t",
		"GIT_AUTHOR_EMAIL=t@t",
		"GIT_COMMITTER_NAME=t",
		"GIT_COMMITTER_EMAIL=t@t",
		"HOME="+work,
	)

	run := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = baseEnv
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v (dir=%s) failed: %v\n%s", args, dir, err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Bare "origin" with -b master so its HEAD points to master.
	if err := os.MkdirAll(bareDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(bareDir, "init", "-q", "--bare", "-b", "master")

	// Clone.
	if err := os.MkdirAll(cloneDir, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "clone", "-q", bareDir, cloneDir)
	run(cloneDir, "config", "commit.gpgsign", "false")

	// Initial commit on master.
	if err := os.WriteFile(filepath.Join(cloneDir, "base.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "base.txt")
	run(cloneDir, "commit", "-q", "-m", "base commit")

	// Push master → establishes origin/master and advances the bare's HEAD.
	run(cloneDir, "push", "-q", "origin", "master")

	// Set the remote tracking HEAD so resolveDefaultBranch can read the symref.
	run(cloneDir, "remote", "set-head", "origin", "master")

	// feat/integrada: at master's tip (no extra commits) → no_own_work → delete.
	run(cloneDir, "branch", "feat/integrada")

	// feat/aberta: one extra commit → pending_work → keep.
	// Non-doc extension (.txt) to avoid review_doc_config reclassification.
	run(cloneDir, "checkout", "-q", "-b", "feat/aberta")
	if err := os.WriteFile(filepath.Join(cloneDir, "aberta.txt"), []byte("aberta\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run(cloneDir, "add", "aberta.txt")
	run(cloneDir, "commit", "-q", "-m", "feat/aberta work")

	// Return to master — it becomes currentBranch; prune must exclude it as the default branch.
	run(cloneDir, "checkout", "-q", "master")

	// gitExec closure: real git, locked to cloneDir, with the same isolated env.
	gitExec := func(args ...string) (string, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = cloneDir
		cmd.Env = baseEnv
		var stderr strings.Builder
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				if exitErr, ok := err.(*exec.ExitError); ok {
					msg = fmt.Sprintf("git %s exited with %d", strings.Join(args, " "), exitErr.ExitCode())
				} else {
					msg = err.Error()
				}
			}
			return strings.TrimSpace(string(out)), fmt.Errorf("%s", msg)
		}
		return strings.TrimSpace(string(out)), nil
	}

	// Guard 1: symref must resolve to origin/master (fixture integrity).
	symref, err := gitExec("symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err != nil || symref != "origin/master" {
		t.Fatalf("fixture: expected symref=origin/master, got %q (err=%v); resolveDefaultBranch symref path depends on this", symref, err)
	}

	// Guard 2: origin/main must NOT exist — without this guard the refusal would be vacuous.
	if _, err := gitExec("rev-parse", "--verify", "-q", "origin/main"); err == nil {
		t.Fatal("fixture: origin/main must not exist in this master-only repo — the 'not found' refusal would be vacuous")
	}

	var outBuf strings.Builder
	deps := branchPruneDeps{
		gitExec:           gitExec,
		ghExec:            nil, // no forge; symref is the only resolution source
		listLocalBranches: defaultListLocalBranches,
		currentBranch:     defaultCurrentBranchForPrune,
		worktreeBranches:  defaultWorktreeBranches,
		deleteBranch: func(_ func(args ...string) (string, error), name string) error {
			t.Errorf("dry-run: deleteBranch must not be called, was called with %q", name)
			return nil
		},
		out: &writerFromBuilder{&outBuf},
	}

	if err := runBranchPrune(false, deps); err != nil {
		t.Fatalf("runBranchPrune returned error: %v\nOutput:\n%s", err, outBuf.String())
	}
	got := outBuf.String()

	// (a) No refusal — the command must not print the "not found" refusal message.
	if strings.Contains(got, "Refusing") {
		t.Errorf("(a) command refused — origin/master was not resolved correctly:\n%s", got)
	}

	// (b) Header names origin/master as the comparison target.
	if !strings.Contains(got, "origin/master") {
		t.Errorf("(b) header must mention origin/master:\n%s", got)
	}

	// Helper: find the output line for a given branch name (fields[0] == name).
	findLine := func(name string) string {
		for _, line := range strings.Split(got, "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == name {
				return line
			}
		}
		return ""
	}

	// (c) master: keep, reason "default branch".
	masterLine := findLine("master")
	if masterLine == "" {
		t.Fatalf("(c) master branch line not found in output:\n%s", got)
	}
	if mf := strings.Fields(masterLine); len(mf) < 2 || mf[1] != "keep" {
		t.Errorf("(c) master must be keep, got: %q", masterLine)
	}
	if !strings.Contains(masterLine, "default branch") {
		t.Errorf("(c) master reason must contain 'default branch', got: %q", masterLine)
	}

	// (d) feat/integrada: delete; reason names origin/master (proves resolved ref reached heuristic).
	integradaLine := findLine("feat/integrada")
	if integradaLine == "" {
		t.Fatalf("(d) feat/integrada line not found in output:\n%s", got)
	}
	if iFields := strings.Fields(integradaLine); len(iFields) < 2 || iFields[1] != "delete" {
		t.Errorf("(d) feat/integrada must be delete, got: %q", integradaLine)
	}
	if !strings.Contains(integradaLine, "origin/master") {
		t.Errorf("(d) feat/integrada reason must name origin/master (proves resolved ref reached heuristic), got: %q", integradaLine)
	}

	// (e) feat/aberta: keep (pending work).
	abertaLine := findLine("feat/aberta")
	if abertaLine == "" {
		t.Fatalf("(e) feat/aberta line not found in output:\n%s", got)
	}
	if aFields := strings.Fields(abertaLine); len(aFields) < 2 || aFields[1] != "keep" {
		t.Errorf("(e) feat/aberta must be keep, got: %q", abertaLine)
	}
}

// ─── Stub tests (resolveDefaultBranch / evaluateBranchWithForge) ─────────────────────────────

// TestDefaultBranch_MasterRepo_NoForge asserts: on a repo whose default branch is "master"
// (symref returns "origin/master", no forge), branch prune evaluates against origin/master,
// excludes the "master" branch as the default, and does NOT refuse with "not found".
// Reconciliation: resolveDefaultBranch picks up "master" from the symref when ghExec is nil, so
// runBranchPrune uses "origin/master" as the rev-parse target and "master" as the exclusion.
// With the old hardcoded "origin/main" this test would fail: localName would be "main" ≠ "master".
func TestDefaultBranch_MasterRepo_NoForge(t *testing.T) {
	t.Parallel()

	// Simulate: symbolic-ref --short refs/remotes/origin/HEAD → "origin/master"
	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			return "origin/master\n", nil
		default:
			return "", fmt.Errorf("unexpected gitExec in master-repo test: %v", args)
		}
	}

	localName, remoteRef := resolveDefaultBranch(gitExec, nil)
	if localName != "master" {
		t.Fatalf("want localName=master, got %q", localName)
	}
	if remoteRef != "origin/master" {
		t.Fatalf("want remoteRef=origin/master, got %q", remoteRef)
	}
}

// TestDefaultBranch_ForgeReturnsTrunk_FilterUsesIt asserts: when the forge API returns
// default_branch="trunk" and there is a MERGED PR with baseRefName="trunk", evaluateBranchWithForge
// correctly classifies the branch as merged_pr (delete), because the filter uses "trunk" (the
// resolved defaultLocalName) instead of the hardcoded "main".
// Reconciliation: forgeDefaultBranch returns "trunk"; that name reaches the baseRefName filter;
// the PR matches; decision is merged_pr. With old hardcoded "main" filter the PR would be
// discarded and the decision would fall back to the content heuristic (not merged_pr).
func TestDefaultBranch_ForgeReturnsTrunk_FilterUsesIt(t *testing.T) {
	t.Parallel()

	prHead := "trunkhead111"

	// forge returns default_branch=trunk for the repo, plus the PR listing for queryForgePRs
	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "remote get-url origin":
			// used by forgeDefaultBranch inside resolveDefaultBranch
			return "https://github.com/acme/repo\n", nil
		case joined == "rev-parse feat/trunk-feature":
			// evaluateBranchWithForge fetches the local tip via `git rev-parse <ref>`
			return prHead + "\n", nil
		case joined == "cat-file -e "+prHead+"^{commit}":
			// objectExists check: commit object is present locally
			return "", nil
		case joined == "merge-base --is-ancestor "+prHead+" "+prHead:
			// tip == prHead → tip IS an ancestor of prHead → case 1 → merged_pr
			return "", nil // exit 0
		default:
			return "", fmt.Errorf("unexpected gitExec in trunk-forge test: %v", args)
		}
	}

	ghExec := func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "api repos/acme/repo":
			return []byte(`{"default_branch":"trunk"}`), nil
		default:
			return nil, fmt.Errorf("unexpected ghExec in trunk-forge test: %v", args)
		}
	}

	localName, remoteRef := resolveDefaultBranch(gitExec, ghExec)
	if localName != "trunk" {
		t.Fatalf("want localName=trunk, got %q", localName)
	}
	if remoteRef != "origin/trunk" {
		t.Fatalf("want remoteRef=origin/trunk, got %q", remoteRef)
	}

	// Verify that evaluateBranchWithForge uses "trunk" as the base-ref filter.
	snapshot := makeSnapshotWith(
		forgePR{Number: 42, State: "MERGED", HeadRefName: "feat/trunk-feature", HeadRefOid: prHead, BaseRefName: "trunk"},
	)
	eval := evaluateBranchWithForge("feat/trunk-feature", "feat/trunk-feature", "origin/feat/trunk-feature", localName, snapshot, gitExec)
	if eval.Decision != branchPruneDecisionMergedPR {
		t.Fatalf("want merged_pr, got %q (%s)", eval.Decision, eval.Reason)
	}
}

// TestDefaultBranch_ForgeVsSymref_ForgePrevails asserts: when the forge says "trunk" and the
// symref says "main", the resolved name is "trunk" — no refusal on divergence.
// Reconciliation: forgeDefaultBranch is called first and returns "trunk"; resolveDefaultBranch
// returns immediately without consulting the symref. Forge is authoritative (vault note).
func TestDefaultBranch_ForgeVsSymref_ForgePrevails(t *testing.T) {
	t.Parallel()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "remote get-url origin":
			return "https://github.com/acme/repo\n", nil
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			// symref says "main" — should be ignored when forge returns "trunk"
			return "origin/main\n", nil
		default:
			return "", fmt.Errorf("unexpected gitExec in forge-vs-symref test: %v", args)
		}
	}

	ghExec := func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "api repos/acme/repo":
			return []byte(`{"default_branch":"trunk"}`), nil
		default:
			return nil, fmt.Errorf("unexpected ghExec in forge-vs-symref test: %v", args)
		}
	}

	localName, remoteRef := resolveDefaultBranch(gitExec, ghExec)
	if localName != "trunk" {
		t.Fatalf("forge must prevail over symref: want localName=trunk, got %q", localName)
	}
	if remoteRef != "origin/trunk" {
		t.Fatalf("forge must prevail over symref: want remoteRef=origin/trunk, got %q", remoteRef)
	}
}

// TestDefaultBranch_NeitherAvailable_FallsBackToMain asserts: when ghExec is nil and the symref
// call fails, resolveDefaultBranch falls back to the literal "main"/"origin/main".
// Reconciliation: both forge and symref paths return errors; fallback constants are returned
// unchanged. This matches the pre-A7 behavior for repositories that actually use "main".
func TestDefaultBranch_NeitherAvailable_FallsBackToMain(t *testing.T) {
	t.Parallel()

	gitExec := func(args ...string) (string, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "symbolic-ref --short refs/remotes/origin/HEAD":
			return "", fmt.Errorf("no symref configured")
		default:
			return "", fmt.Errorf("unexpected gitExec in neither-available test: %v", args)
		}
	}

	localName, remoteRef := resolveDefaultBranch(gitExec, nil)
	if localName != branchPruneDefaultLocalName {
		t.Fatalf("want fallback localName=%q, got %q", branchPruneDefaultLocalName, localName)
	}
	if remoteRef != branchPruneDefaultRemoteRef {
		t.Fatalf("want fallback remoteRef=%q, got %q", branchPruneDefaultRemoteRef, remoteRef)
	}
}
