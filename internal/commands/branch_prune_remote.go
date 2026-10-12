package commands

// branch_prune_remote.go — remote-branch prune (ROADMAP-2026-10-10 ML-1B).
//
// Implements `trackfw branch prune --remote`:
//   - D1: opt-in via deps.remote; no-op when false.
//   - D2: population from `git for-each-ref refs/remotes/origin/`; HEAD and the
//         default branch are excluded (A8).
//   - D3/A6: only `decision == merged_pr` authorises remote deletion; deletable()
//            is never used for the remote path.
//   - D4/A5: preconditions that block ALL remote deletions in the run.
//   - D5: force-with-lease on the evaluated SHA (TOCTOU protection).
//   - D6/A4: protected branches excluded; fail-closed on query failure.
//   - A3: branches that are the base of an open PR are excluded.
//
// ADR: docs/adr/ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md
// Wave 0: docs/seguranca/2026-10-12-wave0-prune-remoto.md

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/kgsaran/trackfw/internal/netdeadline"
)

// remoteBranchEntry holds the short name and evaluated tip SHA for a remote branch
// that received a merged_pr decision. The SHA is captured once during evaluation
// and reused as the --force-with-lease expectation (D5, ADR-2026-10-12).
type remoteBranchEntry struct {
	name string
	sha  string
}

// runRemotePrune implements the `--remote` section of `trackfw branch prune`.
// It is called by runBranchPrune (branch_prune.go) only when deps.remote == true.
//
// Parameters:
//   - apply: whether to perform deletions (false = dry-run report only).
//   - fetchFailed, fetchTimedOut: outcomes of the git fetch in runBranchPrune (D4).
//   - defaultLocalName: resolved default branch name (e.g. "main", "master").
//   - defaultFromForge: whether defaultLocalName came from the forge API (D4 third bullet).
//   - snapshot: forge PR snapshot from queryForgePRs; nil means degraded/truncated (D4).
//   - deps: injectable dependencies (gitExec, ghExec, out).
func runRemotePrune(
	apply bool,
	fetchFailed, fetchTimedOut bool,
	defaultLocalName string,
	defaultFromForge bool,
	snapshot *prSnapshot,
	deps branchPruneDeps,
) error {
	fmt.Fprintf(deps.out, "\n--- remote branches ---\n\n")

	// ── D4/A5: Collect preconditions that block ALL remote deletions ──────────────
	var blockCauses []string

	// P1: fetch failed or timed out (D4 first bullet).
	if fetchTimedOut {
		blockCauses = append(blockCauses, "git fetch timed out ("+netdeadline.NetworkTimeoutEnv+")")
	} else if fetchFailed {
		blockCauses = append(blockCauses, "git fetch failed")
	}

	// P2: forge snapshot unavailable — degraded or truncated (D4 second bullet).
	if snapshot == nil {
		blockCauses = append(blockCauses, "forge PR snapshot unavailable (forge degraded or truncated)")
	}

	// P3: default branch not from forge (D4 third bullet, ADR-2026-10-12).
	if !defaultFromForge {
		blockCauses = append(blockCauses, "default branch could not be read from forge API")
	}

	// A5: fetch URL ≠ push URL → remote deletion would target a different repository.
	if fetchURL, err := deps.gitExec("remote", "get-url", "origin"); err == nil {
		if pushURL, err2 := deps.gitExec("remote", "get-url", "--push", "origin"); err2 != nil {
			blockCauses = append(blockCauses, "could not read push URL for origin")
		} else if strings.TrimSpace(pushURL) != strings.TrimSpace(fetchURL) {
			blockCauses = append(blockCauses, "fetch URL and push URL for origin differ — remote deletion disabled (A5)")
		}
	} else {
		blockCauses = append(blockCauses, "could not read remote URL for origin")
	}

	// A4: protected branches — fail-closed.
	protectedBranches, protErr := queryProtectedBranches(deps.gitExec, deps.ghExec)
	if protErr != nil {
		blockCauses = append(blockCauses, fmt.Sprintf("could not read protected branches: %v", protErr))
	}

	// Report precondition failures and stop (both dry-run and apply, per AC7 requirement
	// that dry-run must be truthful about what would actually happen).
	if len(blockCauses) > 0 {
		for _, c := range blockCauses {
			fmt.Fprintf(deps.out, "remote deletion blocked: %s\n", c)
		}
		fmt.Fprintln(deps.out, "[dry-run] would delete 0 remote branch(es)")
		return nil
	}

	// ── D2/A8: List remote branches, excluding HEAD and the default branch ─────────
	refsRaw, err := deps.gitExec("for-each-ref", "--format=%(refname)", "refs/remotes/origin/")
	if err != nil {
		fmt.Fprintf(deps.out, "could not list remote tracking refs: %v\n", err)
		return err
	}

	var remoteBranches []string
	for _, line := range strings.Split(refsRaw, "\n") {
		ref := strings.TrimSpace(line)
		if ref == "" {
			continue
		}
		// A8: exclude by full refname — %(refname:short) of HEAD is "origin" which would
		// not match a filter on "origin/HEAD"; the full refname is reliable.
		if ref == "refs/remotes/origin/HEAD" {
			continue
		}
		b := strings.TrimPrefix(ref, "refs/remotes/origin/")
		// Exclude the default branch (D2).
		if b == defaultLocalName {
			continue
		}
		remoteBranches = append(remoteBranches, b)
	}
	sort.Strings(remoteBranches)

	fmt.Fprintf(deps.out, "trackfw branch prune --remote — evaluating %d remote branch(es) against the forge\n\n", len(remoteBranches))

	// ── A3: Build the set of branch names that are bases of open PRs ──────────────
	// A branch that is the base of an open PR must not be deleted: removing it would
	// close the dependent PR on GitHub. This applies regardless of isCrossRepository
	// because a fork PR targeting a base in origin also closes if the base disappears
	// (Wave 0, A3, advisor note).
	openPRBases := map[string]bool{}
	for i := range snapshot.prs {
		if snapshot.prs[i].State == "OPEN" {
			openPRBases[snapshot.prs[i].BaseRefName] = true
		}
	}

	// ── D3/A6: Evaluate each remote branch ───────────────────────────────────────
	var toDelete []remoteBranchEntry
	for _, b := range remoteBranches {
		ref := "refs/remotes/origin/" + b

		// D6/A4: protected branch → keep.
		if protectedBranches[b] {
			fmt.Fprintf(deps.out, "  %-40s keep    protected branch\n", b)
			continue
		}

		// A3: base of an open PR → keep.
		if openPRBases[b] {
			fmt.Fprintf(deps.out, "  %-40s keep    base of open PR\n", b)
			continue
		}

		// D5: read the tip SHA once — this is the SHA used for both evaluation and the
		// force-with-lease. A second read at deletion time would open a TOCTOU window
		// (Wave 0, Seção 3, braço C).
		sha, err := deps.gitExec("rev-parse", "--verify", ref)
		if err != nil {
			fmt.Fprintf(deps.out, "  %-40s keep    could not resolve tip: %v\n", b, err)
			continue
		}
		sha = strings.TrimSpace(sha)

		// D3/A6: evaluate using the forge signal, operating on the tip SHA.
		// Pass sha as ref so that gitExec("rev-parse", sha) inside the function
		// returns sha directly — the evaluation anchors to the remote tip.
		// Pass "origin/"+b as upstream sentinel (non-empty) so that case 4
		// (never-pushed guard) does not fire for remote branches.
		eval := evaluateBranchWithForge(sha, b, "origin/"+b, defaultLocalName, snapshot, deps.gitExec)

		// A6 (critical): only merged_pr authorises remote deletion.
		// deletable() also accepts no_own_work and content_identical (heuristic decisions);
		// using deletable() here would violate ADR D3.
		if eval.Decision != branchPruneDecisionMergedPR {
			fmt.Fprintf(deps.out, "  %-40s keep    %s\n", b, eval.Reason)
			continue
		}

		fmt.Fprintf(deps.out, "  %-40s delete  %s\n", b, eval.Reason)
		toDelete = append(toDelete, remoteBranchEntry{name: b, sha: sha})
	}

	fmt.Fprintln(deps.out)

	// ── Dry-run or apply ──────────────────────────────────────────────────────────
	if !apply {
		if len(toDelete) == 0 {
			fmt.Fprintln(deps.out, "[dry-run] nothing to delete (remote).")
		} else {
			names := make([]string, len(toDelete))
			for i, e := range toDelete {
				names[i] = e.name
			}
			fmt.Fprintf(deps.out, "[dry-run] would delete %d remote branch(es): %s. Rerun with --apply to delete.\n",
				len(toDelete), strings.Join(names, ", "))
		}
		return nil
	}

	if len(toDelete) == 0 {
		fmt.Fprintln(deps.out, "nothing to delete (remote).")
		return nil
	}

	var remoteDeleted []string
	for _, entry := range toDelete {
		// Belt-and-suspenders: reconfirm --apply and --remote immediately before each
		// deletion (ADR D5 "Imediatamente antes de cada remoção").
		if !apply || !deps.remote {
			break
		}
		if err := deleteRemoteBranch(deps.gitExec, entry.name, entry.sha); err != nil {
			if errors.Is(err, netdeadline.ErrTimeout) {
				fmt.Fprintf(deps.out, "skip %s: network timeout — remote state uncertain (%s)\n",
					entry.name, netdeadline.PushTimeoutEnv)
			} else {
				fmt.Fprintf(deps.out, "skip %s: %v\n", entry.name, err)
			}
			continue
		}
		remoteDeleted = append(remoteDeleted, entry.name)
	}

	if len(remoteDeleted) == 0 {
		fmt.Fprintln(deps.out, "deleted 0 remote branch(es).")
	} else {
		fmt.Fprintf(deps.out, "deleted %d remote branch(es): %s\n",
			len(remoteDeleted), strings.Join(remoteDeleted, ", "))
	}
	return nil
}

// deleteRemoteBranch removes branch b from origin using force-with-lease anchored to the
// SHA that was evaluated (D5, ADR-2026-10-12). The lease protects refs/heads/<b> on the
// server: if the remote advanced since the evaluation, the push is rejected and the branch
// is preserved.
//
// The lease refname is refs/heads/<b> (the server-side ref that will be updated/deleted),
// NOT refs/remotes/origin/<b> (the client-side tracking ref). Using the tracking ref would
// make the server check a ref it does not have, vacuously satisfying the lease and opening
// the TOCTOU window (Wave 0, Seção 5, Direção 1).
func deleteRemoteBranch(gitExec func(...string) (string, error), b, sha string) error {
	lease := "refs/heads/" + b + ":" + sha
	_, err := gitExec("push", "--force-with-lease="+lease, "origin", ":refs/heads/"+b)
	return err
}

// queryProtectedBranches returns the set of branch names marked as protected in the
// GitHub repository. Uses `gh api repos/<owner>/<repo>/branches?protected=true&per_page=100
// --paginate` (A4, Wave 0 Seção 4). Fail-closed: any error returns (nil, error).
//
// The --paginate output may be a single JSON array ([...]) or concatenated arrays ([...][...]).
// json.NewDecoder in a loop handles both formats without needing jq preprocessing.
func queryProtectedBranches(gitExec func(...string) (string, error), ghExec ghExecFn) (map[string]bool, error) {
	if ghExec == nil {
		return nil, fmt.Errorf("gh is not available (no forge CLI)")
	}

	remoteURL, err := gitExec("remote", "get-url", "origin")
	if err != nil {
		return nil, fmt.Errorf("could not read origin URL: %v", err)
	}
	host, ownerRepo := parseHostOwnerRepo(strings.TrimSpace(remoteURL))
	if host != "github.com" {
		return nil, fmt.Errorf("forge host %q is not github.com; protected-branch lookup skipped", host)
	}

	endpoint := fmt.Sprintf("repos/%s/branches?protected=true&per_page=100", ownerRepo)
	out, err := ghExec("api", "--paginate", endpoint)
	if err != nil {
		return nil, fmt.Errorf("gh api branches (protected) failed: %v", err)
	}

	type branchInfo struct {
		Name      string `json:"name"`
		Protected bool   `json:"protected"`
	}
	result := map[string]bool{}
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var page []branchInfo
		if err := dec.Decode(&page); err != nil {
			return nil, fmt.Errorf("could not parse protected-branches response: %v", err)
		}
		for _, bi := range page {
			if bi.Protected {
				result[bi.Name] = true
			}
		}
	}
	return result, nil
}
