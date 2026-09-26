package commands

// ML-3A (REQ-2026-09-09, AC12) — D1 of ADR-2026-09-26 at the command layer: `branch new` WRITES the
// link, `commit` READS it. Reconciliation sentences are in each test's doc comment.

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/validator"
)

// Affirms: D1 at the command layer — `branch new` records the branch↔roadmap link, and only AFTER
// git created the branch, so the recorded state never describes a branch that does not exist.
func TestBranchNew_RecordsLinkAfterCheckout(t *testing.T) {
	deps, _, checkoutCalls := makeBranchDeps(true, []string{"ROADMAP-x.md"})
	order := []string{}
	deps.execGitCheckout = func(branchName string) error {
		order = append(order, "checkout:"+branchName)
		*checkoutCalls = append(*checkoutCalls, branchName)
		return nil
	}
	deps.recordLink = func(cfg config.ProjectConfig, branch string) error {
		order = append(order, "record:"+branch)
		return nil
	}
	if err := runBranchNew("feat/minha-feature", false, deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(order) != 2 || order[0] != "checkout:feat/minha-feature" || order[1] != "record:feat/minha-feature" {
		t.Fatalf("link must be recorded after checkout, got %v", order)
	}
}

// Affirms: the link is an accelerator and never a gate — a failure to record it is reported but does
// not turn a successful, governed branch creation into an error.
func TestBranchNew_RecordLinkFailureIsReportedNotFatal(t *testing.T) {
	deps, out, _ := makeBranchDeps(true, []string{"ROADMAP-x.md"})
	deps.recordLink = func(cfg config.ProjectConfig, branch string) error {
		return fmt.Errorf("disk on fire")
	}
	if err := runBranchNew("feat/minha-feature", false, deps); err != nil {
		t.Fatalf("recording failure must not fail the command, got %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("disk on fire")) {
		t.Fatalf("the failure must be reported to the user, got %q", out.String())
	}
}

// Affirms: the gate still precedes everything — a blocked branch neither checks out nor records a
// link, so a rejected branch leaves no state behind.
func TestBranchNew_BlockedRecordsNoLink(t *testing.T) {
	deps, _, _ := makeBranchDeps(false, []string{"ROADMAP-outra.md"})
	recorded := 0
	deps.recordLink = func(cfg config.ProjectConfig, branch string) error { recorded++; return nil }
	if err := runBranchNew("feat/minha-feature", false, deps); err == nil {
		t.Fatal("expected the gate to block")
	}
	if recorded != 0 {
		t.Fatalf("a blocked branch must record no link, got %d recordings", recorded)
	}
}

// Affirms: D1 at commit time — the written link ADDS acceptance: a branch whose roadmap was renamed
// out of inference reach stays governed because the link recorded at creation still names a roadmap
// present in wip/ or done/.
func TestCommit_WrittenLinkAcceptsWhenInferenceFails(t *testing.T) {
	deps, out, calls := makeCommitDeps("feat/minha-feature", false, []string{"ROADMAP-renomeado.md"})
	deps.branchLink = func(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) validator.BranchLinkStatus {
		return validator.BranchLinkStatus{Roadmap: "ROADMAP-renomeado.md", Present: true, InScope: true}
	}
	if err := runCommit("fix(x): y", deps); err != nil {
		t.Fatalf("the written link must keep the branch governed: %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("commit must have been executed, got %v", *calls)
	}
	if !bytes.Contains(out.Bytes(), []byte("written link")) {
		t.Fatalf("the acceptance source must be stated, got %q", out.String())
	}
}

// Affirms: the ADR's forbidden answer is not taken — when the link is STALE and inference also
// fails, the block names the stale target instead of degrading silently.
func TestCommit_StaleLinkIsNamedWhenBlocking(t *testing.T) {
	deps, out, calls := makeCommitDeps("feat/minha-feature", false, []string{"ROADMAP-outra.md"})
	deps.branchLink = func(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) validator.BranchLinkStatus {
		return validator.BranchLinkStatus{Roadmap: "ROADMAP-que-saiu-de-wip.md", Present: true}
	}
	if err := runCommit("fix(x): y", deps); err == nil {
		t.Fatal("expected the commit to be blocked")
	}
	if len(*calls) != 0 {
		t.Fatalf("no commit must be executed, got %v", *calls)
	}
	if !bytes.Contains(out.Bytes(), []byte("ROADMAP-que-saiu-de-wip.md")) {
		t.Fatalf("the stale target must be named, got %q", out.String())
	}
}

// Affirms: the link is only consulted when inference fails — a branch accepted by inference never
// reads the link file, so the accelerator adds no behaviour to the path that already worked.
func TestCommit_LinkNotConsultedWhenInferenceMatches(t *testing.T) {
	deps, _, calls := makeCommitDeps("feat/minha-feature", true, nil)
	consulted := 0
	deps.branchLink = func(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) validator.BranchLinkStatus {
		consulted++
		return validator.BranchLinkStatus{}
	}
	if err := runCommit("fix(x): y", deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if consulted != 0 {
		t.Fatalf("the link must not be read when inference already matched, got %d reads", consulted)
	}
	if len(*calls) != 1 {
		t.Fatalf("commit must have been executed, got %v", *calls)
	}
}
