package commands

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// forgePR holds the PR fields returned by `gh pr list --json ...`.
// Field names mirror gh's camelCase JSON keys exactly.
// headRepositoryOwner is included for completeness and future filtering — the
// primary fork discriminant is isCrossRepository (Cenário 1, Wave 0).
type forgePR struct {
	Number            int    `json:"number"`
	State             string `json:"state"` // "OPEN", "MERGED", "CLOSED"
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	HeadRepositoryOwner struct {
		Login string `json:"login"`
	} `json:"headRepositoryOwner"`
}

// prSnapshot holds the result of a single forge PR list query (D3, ADR-2026-10-03).
// A nil *prSnapshot means the query is degraded (D2) or truncated (A7) — every
// branch falls back to the content heuristic and no forge-signal delete fires.
type prSnapshot struct {
	prs []forgePR
}

// forgeQueryLimit is the maximum items requested from `gh pr list`.
// If the response contains exactly this many items, the query is treated as
// incomplete (A7, ADR-2026-10-03): no branch receives a forge-signal decision
// and a one-line cause is printed to the report. Chosen to be well above any
// known acervo (428 PRs measured 2026-10-03 in ~2.3 s) while still bounding
// the response time. See A8: any limit < 1000 would leave < 17% margin.
const forgeQueryLimit = 3000

// ghExecFn is the injectable gh CLI runner. It receives gh sub-command arguments
// (e.g. "pr", "list", "--state", "all") and returns raw stdout bytes.
//
// nil means "no forge CLI available" (D2). Production: defaultGhExec.
// Tests set it to a stub returning pre-baked JSON; the tests never touch the
// network. This is the injection boundary for forge functionality — injecting at
// this level keeps URL parsing, --repo assembly (A1), JSON parsing (A5), the
// truncation check (A7) and the fork filter all inside tested Go code.
type ghExecFn func(args ...string) ([]byte, error)

// defaultGhExec runs `gh <args...>` and returns stdout.
// It is the production implementation of ghExecFn — wired only in RunE of
// branch prune, push and ship; never wired in tests (tests pass nil or a stub).
func defaultGhExec(args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	return cmd.Output()
}

// queryForgePRs performs the single `gh pr list` query (D3, ADR-2026-10-03).
//
// Returns (*prSnapshot, "") on success.
// Returns (nil, reason) on any degradation (D2):
//   - ghExec is nil
//   - origin remote URL cannot be read or parsed
//   - host is not github.com (A1)
//   - gh exits with non-zero (A5)
//   - JSON cannot be parsed (A5)
//   - response has exactly forgeQueryLimit items (A7: treat as truncated / incomplete)
//
// gitExec is used only to run `git remote get-url origin` (A1) — the --repo flag
// pins the query to the parsed repository and prevents GH_REPO misdirection.
// forge.Resolve is intentionally NOT used: its CI-file fallback returns "github"
// when .github/workflows/ exists, even with origin pointing to a non-GitHub host
// (measured in Wave 0, Cenário 5).
func queryForgePRs(gitExec func(...string) (string, error), ghExec ghExecFn) (*prSnapshot, string) {
	if ghExec == nil {
		return nil, "gh is not available"
	}

	remoteURL, err := gitExec("remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Sprintf("could not read git remote get-url origin: %v", err)
	}
	remoteURL = strings.TrimSpace(remoteURL)

	host, ownerRepo := parseHostOwnerRepo(remoteURL)
	switch {
	case host == "":
		return nil, fmt.Sprintf("could not parse origin remote URL %q; forge PR lookup skipped (D2, ADR-2026-10-03)", remoteURL)
	case host != "github.com":
		return nil, fmt.Sprintf("forge host %q is not github.com; forge PR lookup skipped (D2, ADR-2026-10-03)", host)
	}

	// A1: pass --repo HOST/OWNER/REPO to override GH_REPO and any multi-remote resolution.
	repoArg := host + "/" + ownerRepo
	out, err := ghExec(
		"pr", "list",
		"--repo", repoArg,
		"--state", "all",
		"--limit", strconv.Itoa(forgeQueryLimit),
		"--json", "number,state,headRefName,headRefOid,baseRefName,isCrossRepository,headRepositoryOwner",
	)
	if err != nil {
		// A5: any exit ≠ 0 → D2. Do not treat as empty list.
		return nil, fmt.Sprintf("gh pr list --repo %s failed: %v", repoArg, err)
	}

	var prs []forgePR
	if err := json.Unmarshal(out, &prs); err != nil {
		// A5: JSON parse failure → D2. Do not treat as empty list.
		return nil, fmt.Sprintf("could not parse gh pr list output: %v", err)
	}

	// A7: exactly forgeQueryLimit items means the response may be incomplete.
	// Block ALL forge-signal decisions for this run — a branch present in the
	// window might have an OPEN PR outside it, and under case 1 it would be
	// deleted with the MERGED PR's signal while the OPEN PR is invisible.
	if len(prs) == forgeQueryLimit {
		return nil, fmt.Sprintf(
			"gh pr list returned %d items (= limit %d); response may be truncated — "+
				"forge PR signal disabled for this run (A7, ADR-2026-10-03); "+
				"all branches fall back to content heuristic",
			forgeQueryLimit, forgeQueryLimit,
		)
	}

	return &prSnapshot{prs: prs}, ""
}

// parseHostOwnerRepo parses a git remote URL and returns the lowercase hostname
// and "owner/repo" (without .git suffix).
//
// Supported formats:
//   - HTTPS: https://github.com/owner/repo.git
//   - SSH git@ short form: git@github.com:owner/repo.git
//   - SSH long form: ssh://git@github.com/owner/repo.git
//
// Returns ("", "") on unrecognised format or insufficient path components.
// This is a local re-implementation of the host-extraction logic from
// internal/forge/resolve.go (extractHost), extended to also return owner/repo.
// The forge package's extractHost is unexported and not in the allowed file list;
// duplicating the logic here keeps the dependency graph clean.
func parseHostOwnerRepo(rawURL string) (host, ownerRepo string) {
	rawURL = strings.TrimSpace(rawURL)
	var rest string

	switch {
	case strings.HasPrefix(rawURL, "git@"):
		// git@github.com:owner/repo.git
		after := rawURL[4:] // strip "git@"
		idx := strings.IndexByte(after, ':')
		if idx < 0 {
			return "", ""
		}
		host = strings.ToLower(after[:idx])
		rest = after[idx+1:]

	case strings.HasPrefix(rawURL, "ssh://"):
		// ssh://git@github.com/owner/repo.git
		after := rawURL[6:] // strip "ssh://"
		if idx := strings.IndexByte(after, '@'); idx >= 0 {
			after = after[idx+1:]
		}
		idx := strings.IndexByte(after, '/')
		if idx < 0 {
			return "", ""
		}
		host = strings.ToLower(after[:idx])
		rest = after[idx+1:]

	case strings.HasPrefix(rawURL, "https://") || strings.HasPrefix(rawURL, "http://"):
		rawURL = strings.TrimPrefix(rawURL, "https://")
		rawURL = strings.TrimPrefix(rawURL, "http://")
		if idx := strings.IndexByte(rawURL, '@'); idx >= 0 {
			rawURL = rawURL[idx+1:]
		}
		idx := strings.IndexByte(rawURL, '/')
		if idx < 0 {
			return "", ""
		}
		host = strings.ToLower(rawURL[:idx])
		rest = rawURL[idx+1:]

	default:
		return "", ""
	}

	// Strip port from host if present (e.g. "github.com:443" → "github.com").
	if idx := strings.IndexByte(host, ':'); idx >= 0 {
		host = host[:idx]
	}

	// Strip .git suffix from path.
	rest = strings.TrimSuffix(rest, ".git")

	// Ensure we have at least owner/repo.
	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", ""
	}
	return host, parts[0] + "/" + parts[1]
}

// upstreamFor returns the tracking upstream short name for a local branch via
// `git for-each-ref --format=%(upstream:short) refs/heads/<branch>`.
//
// An empty return value means the branch has no upstream (never pushed). This is
// the correct predicate for D1 case 4 (ADR-2026-10-03) — `git rev-parse @{u}`
// is intentionally NOT used: it fails with exit 128 on [gone] branches (A4).
func upstreamFor(branch string, gitExec func(...string) (string, error)) string {
	out, err := gitExec("for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// objectExists runs `git cat-file -e <oid>^{commit}` and reports whether the
// commit object is available in the local object store (A3, ADR-2026-10-03).
//
// Returns false on any error — including "commit not found" (exit 1) and fatal
// errors (exit 128, e.g. malformed object ID). This prevents merge-base
// --is-ancestor from receiving an unknown object, which would return exit 128
// and be indistinguishable from exit 1 ("not an ancestor") in some wrappers.
func objectExists(oid string, gitExec func(...string) (string, error)) bool {
	_, err := gitExec("cat-file", "-e", oid+"^{commit}")
	return err == nil
}

// isNotAncestorError reports whether err represents `git merge-base --is-ancestor`
// returning exit code 1 (not an ancestor), as opposed to a fatal error (exit 128
// or similar). Used to distinguish case 2/2b from eval_error.
//
// defaultGitExec wraps exit-code errors as errors.New("git ... exited with N")
// when stderr is empty; for merge-base --is-ancestor, exit 1 has no stderr output,
// so the message will be "git merge-base --is-ancestor ... exited with 1".
// Fake gitExec implementations in tests use the same convention.
//
// ⚠️  Must use HasSuffix, not Contains. "exited with 128" contains "exited with 1"
// as a substring, which would cause fatal errors to be misclassified as "not ancestor"
// (A3 explicitly forbids confusing exit 1 with exit 128).
func isNotAncestorError(err error) bool {
	if err == nil {
		return false
	}
	return strings.HasSuffix(err.Error(), "exited with 1")
}

// evaluateBranchWithForge applies the D1 signal order from ADR-2026-10-03:
// forge PR signal first (cases 0–4), content heuristic as fallback (case 5).
//
//   - ref: git ref for git operations (e.g. "feat/foo" for prune, "origin/feat/foo"
//     for detectPendingSquashMerges). Passed to merge-base, rev-parse, diff.
//   - prName: branch name for PR headRefName lookup (always the short name, e.g.
//     "feat/foo"). A6: never "origin/feat/foo" — the forge stores the short name.
//   - upstream: pre-computed upstream short name (upstreamFor). Empty means
//     "never pushed". Callers that operate on remote refs (detectPendingSquashMerges)
//     must pass a non-empty sentinel to skip case 4 — remote refs are always pushed
//     by definition and case 4 would misfire if the local branch copy is absent.
//   - snapshot: nil means degraded or truncated (D2/A7) — falls back directly to
//     evaluateBranchIntegration(ref, gitExec). Never produces a new forge-signal
//     decision when nil.
//
// Decision priority among multiple MERGED PRs with valid base (A2):
//  1. delete (any PR whose headRefOid contains the tip — case 1) — returned immediately
//  2. commits_after_merged_pr (tip has commits beyond any PR head — case 2)
//  3. diverged_from_merged_pr (tip and all PR heads are mutually diverged — case 2b)
//  4. merged_head_absent (all checked PR heads missing locally — review, never delete)
func evaluateBranchWithForge(ref, prName, upstream string, snapshot *prSnapshot, gitExec func(...string) (string, error)) branchPruneEvaluation {
	// D2 / A7: snapshot nil → degrade to content heuristic.
	if snapshot == nil {
		return evaluateBranchIntegration(ref, gitExec)
	}

	// Filter PRs for this branch: non-fork, headRefName matches (A1/C1).
	var openPRs []forgePR
	var mergedValidBase []forgePR
	var closedPRs []forgePR
	for _, pr := range snapshot.prs {
		if pr.HeadRefName != prName {
			continue
		}
		if pr.IsCrossRepository {
			// Fork: excluded (Cenário 1, Wave 0). The filter satisfies AC3 but
			// the real safety guard is is-ancestor + baseRefName (see Wave 0 analysis).
			continue
		}
		switch pr.State {
		case "OPEN":
			openPRs = append(openPRs, pr)
		case "MERGED":
			if pr.BaseRefName == branchPruneDefaultLocalName { // A2
				mergedValidBase = append(mergedValidBase, pr)
			}
		case "CLOSED":
			closedPRs = append(closedPRs, pr)
		}
	}

	// Case 0: any OPEN PR (non-fork) → keep. Prevails over any MERGED PR with
	// the same head — the defence against reused branch names (D1 §0).
	if len(openPRs) > 0 {
		return branchPruneEvaluation{
			Name:     ref,
			Decision: branchPruneDecisionOpenPR,
			Reason:   fmt.Sprintf("open PR #%d", openPRs[0].Number),
		}
	}

	// Cases 1, 2, 2b: check MERGED PRs with valid base (A2).
	if len(mergedValidBase) > 0 {
		tip, err := gitExec("rev-parse", ref)
		if err == nil {
			tip = strings.TrimSpace(tip)

			// Track the best result across all merged PRs.
			// Priority: delete (1) > commitsAfter (2) > diverged (2b) > headAbsent (review).
			type mergedOutcome int
			const (
				mergedOutcomeNone       mergedOutcome = iota
				mergedOutcomeHeadAbsent               // head commit missing locally
				mergedOutcomeDiverged                 // case 2b
				mergedOutcomeCommits                  // case 2
				// delete (case 1) returns immediately; no enum slot needed.
			)
			best := mergedOutcomeNone
			var bestPR forgePR

			for _, pr := range mergedValidBase {
				// A3: cat-file before is-ancestor to distinguish exit 1 (not ancestor)
				// from exit 128 (fatal — object missing). Without this, a missing
				// headRefOid causes is-ancestor to return exit 128, which a wrapper
				// that only checks err != nil would treat as "not ancestor" (case 2/2b)
				// instead of "head not available" (review).
				if !objectExists(pr.HeadRefOid, gitExec) {
					if best < mergedOutcomeHeadAbsent {
						best = mergedOutcomeHeadAbsent
						bestPR = pr
					}
					continue
				}

				// Case 1: is tip an ancestor of (or equal to) the PR head?
				// i.e. the merged PR contains the tip — safe to delete.
				_, err1 := gitExec("merge-base", "--is-ancestor", tip, pr.HeadRefOid)
				if err1 == nil {
					// Case 1: delete immediately (one PR is enough — D1 §1).
					return branchPruneEvaluation{
						Name:     ref,
						Decision: branchPruneDecisionMergedPR,
						Reason:   fmt.Sprintf("merged PR #%d contains this branch", pr.Number),
					}
				}
				if !isNotAncestorError(err1) {
					// Unexpected error (exit 128, etc.) → treat as head absent.
					if best < mergedOutcomeHeadAbsent {
						best = mergedOutcomeHeadAbsent
						bestPR = pr
					}
					continue
				}

				// Case 2: is the PR head an ancestor of the tip?
				// i.e. the tip has commits beyond the merged PR head — pending work.
				_, err2 := gitExec("merge-base", "--is-ancestor", pr.HeadRefOid, tip)
				if err2 == nil {
					if best < mergedOutcomeCommits {
						best = mergedOutcomeCommits
						bestPR = pr
					}
					continue
				}
				if !isNotAncestorError(err2) {
					if best < mergedOutcomeHeadAbsent {
						best = mergedOutcomeHeadAbsent
						bestPR = pr
					}
					continue
				}

				// Neither is ancestor → case 2b: diverged (e.g. rebase or amend after merge).
				if best < mergedOutcomeDiverged {
					best = mergedOutcomeDiverged
					bestPR = pr
				}
			}

			switch best {
			case mergedOutcomeCommits:
				return branchPruneEvaluation{
					Name:     ref,
					Decision: branchPruneDecisionCommitsAfterMerged,
					Reason:   fmt.Sprintf("commits after the merged PR #%d", bestPR.Number),
				}
			case mergedOutcomeDiverged:
				return branchPruneEvaluation{
					Name:     ref,
					Decision: branchPruneDecisionDivergedFromMerged,
					Reason:   fmt.Sprintf("diverged from the merged PR #%d", bestPR.Number),
				}
			case mergedOutcomeHeadAbsent:
				return branchPruneEvaluation{
					Name:     ref,
					Decision: branchPruneDecisionMergedHeadAbsent,
					Reason:   fmt.Sprintf("merged PR #%d: head commit not available locally — review before deleting", bestPR.Number),
				}
			}
			// mergedOutcomeNone: all merged PRs were skipped by some earlier continue
			// that did not set best (shouldn't happen since len(mergedValidBase) > 0).
		}
		// rev-parse failed (bad ref, etc.): fall through to case 3/4/5.
	}

	// Case 3: CLOSED PR without merge → review, never deleted automatically.
	if len(closedPRs) > 0 {
		return branchPruneEvaluation{
			Name:     ref,
			Decision: branchPruneDecisionClosedPR,
			Reason:   fmt.Sprintf("PR closed without merge #%d", closedPRs[0].Number),
		}
	}

	// Case 4: no upstream — the branch was never pushed. 🔴 Never deleted, even if
	// content is identical (D1 §4, ADR-2026-10-03). The upstream check alone is the
	// correct predicate: by this point every case that could delete a branch (case 0
	// open_pr, cases 1/2/2b/head-absent from MERGED PRs with valid base, case 3
	// closed_pr) has already returned. A non-fork MERGED PR whose base is not main is
	// not in mergedValidBase, so its existence does NOT imply the work was integrated
	// into main — and the local branch may be the only copy (AJ1, Wave 2 review).
	// hasPRs is NOT checked here: the guard must fire for any branch with upstream==""
	// regardless of whether it has PRs with a non-main base (stacked PRs etc.).
	// Callers that iterate remote refs (detectPendingSquashMerges) must pass a
	// non-empty upstream sentinel so this case never fires for remote branches.
	if upstream == "" {
		return branchPruneEvaluation{
			Name:     ref,
			Decision: branchPruneDecisionNoPRNeverPushed,
			Reason:   "never pushed, no PR",
		}
	}

	// Case 5: no forge signal — fall back to the content heuristic (today's
	// behaviour: evaluateBranchIntegration). Covers:
	//   - branches with no PRs but with an upstream (were pushed, no PR opened)
	//   - branches whose non-fork PRs all have base ≠ main AND have an upstream
	//     (stacked PRs etc. — not considered "merged into main"). Without an
	//     upstream, case 4 has already returned above (AJ1, Wave 2 review).
	return evaluateBranchIntegration(ref, gitExec)
}
