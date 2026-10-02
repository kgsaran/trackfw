package validator

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kgsaran/trackfw/internal/config"
	"github.com/kgsaran/trackfw/internal/pathguard"
)

// BranchLinkFileName is the file that records the WRITTEN branch↔roadmap link (D1 of
// ADR-2026-09-26-precisao-do-vinculo-branch-roadmap-escrever-em-vez-de-inferir).
//
// 🔴 Why a written link exists at all: `trackfw branch new` KNOWS which roadmap is in wip/ at the
// instant it creates the branch. Inferring it back later from the two names is reconstructing
// information that existed and was thrown away — and every inference candidate (substring, word
// boundary, token overlap) is an approximation of something that does not need approximating.
//
// It lives next to .trackfw-attention.json, in the roadmap directory, and for the same reason: it is
// per-checkout coordination state, not a governance artifact. Deliberately NOT the roadmap
// frontmatter — `status:` there is synced by `roadmap move`, parsed by internal/roadmapdoc and
// pinned by the barrier contract, so writing a branch name into it during `branch new` would put an
// uncommitted edit to a governance artifact in play on every branch creation.
//
// Its absence is never an error: a clone, a fork or a `git checkout -b` never produces one, which is
// exactly the population D1 keeps the inference fallback for.
const BranchLinkFileName = ".trackfw-branch-links.json"

// branchLinkFile is the on-disk shape: {"version":1,"links":{"feat/slug":"ROADMAP-….md"}}.
type branchLinkFile struct {
	Version int               `json:"version"`
	Links   map[string]string `json:"links"`
}

const branchLinkFileVersion = 1

// BranchLinkPath returns the absolute-or-relative path of the link file for cfg.
func BranchLinkPath(cfg config.ProjectConfig) string {
	return filepath.Join(cfg.RoadmapDir, BranchLinkFileName)
}

// readBranchLinks returns the recorded links. A missing, unreadable or malformed file yields an
// empty map and no error: the link is an accelerator, never a gate — a corrupt file must degrade to
// inference, not block `commit`/`ship`.
func readBranchLinks(cfg config.ProjectConfig) map[string]string {
	data, err := readRegularFile(BranchLinkPath(cfg))
	if err != nil {
		return nil
	}
	var file branchLinkFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil
	}
	return file.Links
}

// BranchLinkStatus is the state of the written link for one branch.
type BranchLinkStatus struct {
	// Roadmap is the recorded roadmap filename ("" when no link is recorded).
	Roadmap string
	// Present is true when a link is recorded for the branch.
	Present bool
	// InScope is true when the recorded roadmap is still in wip/, blocked/ or done/.
	// Present && !InScope is a STALE link — the roadmap was renamed, moved out of
	// wip/+blocked/+done/, or deleted. The ADR forbids answering a stale link with a silent
	// fallback (D4 of ADR-2026-10-01).
	InScope bool
}

// BranchLinkFor resolves the written link for branch against the roadmaps currently in
// wipDirs, blocked/ (derived from cfg) and doneDirs. It is the single reader of the link file —
// `validate`, `commit` and `ship` all go through here.
//
// D4 (ADR-2026-10-01): the scope was expanded from wip/+done/ to wip/+blocked/+done/ so that a
// branch whose roadmap was moved to blocked/ is not reported as STALE. The blocked dirs are
// resolved internally from cfg without changing the public signature — commit.go assigns
// BranchLinkFor to a typed func field with the original 4-arg signature.
func BranchLinkFor(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) BranchLinkStatus {
	recorded := strings.TrimSpace(readBranchLinks(cfg)[branch])
	if recorded == "" {
		return BranchLinkStatus{}
	}
	status := BranchLinkStatus{Roadmap: recorded, Present: true}
	// Scope: wip/ ∪ blocked/ ∪ done/ (D4).
	blockedDirs := resolveBlockedDirs(cfg)
	allDirs := append(append(append([]string{}, wipDirs...), blockedDirs...), doneDirs...)
	for _, dir := range allDirs {
		entries, _ := listDir(dir)
		for _, name := range entries {
			if name == recorded {
				status.InScope = true
				return status
			}
		}
	}
	return status
}

// BranchLinkStaleWarning is the message emitted when a link names a roadmap that is no longer in
// wip/, blocked/ nor done/. It is a WARNING and never a violation: promoting it would break the
// additive order of D4 of ADR-2026-10-01 — a branch that passes today by inference must not
// start failing because a stale accelerator entry exists next to it. Silence is what the ADR
// forbids, not leniency.
func BranchLinkStaleWarning(cfg config.ProjectConfig, branch, roadmap string) string {
	return fmt.Sprintf(
		"branch_link_stale: the written link for branch %q names roadmap %q, which is no longer in wip/, blocked/ nor done/ — falling back to name inference. Re-create the link with 'trackfw branch new', or drop the entry from %s",
		branch, roadmap, BranchLinkPath(cfg),
	)
}

// RecordBranchLink writes the branch↔roadmap link for branch, resolving the roadmap by inference
// at the moment of creation — which is the moment the information is still exact.
//
// D1 (ADR-2026-10-01): resolves ONLY against wip/. The old implementation also searched done/,
// which could write a permanent link to a completed roadmap for a new branch that happened to
// share its slug — promoting a spurious done/ match to a source of truth. Creating a new
// feat/fix/refactor branch over concluded work always requires `trackfw roadmap move <name> wip`
// first, so there is never a legitimate case for a link to a done/ roadmap at creation time.
//
// It records ONLY when the inference identifies exactly ONE roadmap in wip/. With two or more,
// there is no single truth to write. With none, `branch new` has already blocked.
func RecordBranchLink(cfg config.ProjectConfig, branch string) error {
	slug := NormalizeBranchSlug(branchSlugOf(branch))
	wipDirs := ResolveWIPDirs(cfg)
	// Resolve against wip/ only (D1) — no doneDirs passed.
	matches, _ := MatchRoadmapsForBranchSlug(slug, wipDirs, nil)
	if len(matches) != 1 {
		return nil
	}
	return writeBranchLink(cfg, branch, matches[0])
}

// writeBranchLink merges one entry into the link file, preserving the other branches' entries.
func writeBranchLink(cfg config.ProjectConfig, branch, roadmap string) error {
	links := readBranchLinks(cfg)
	if links == nil {
		links = map[string]string{}
	}
	links[branch] = roadmap

	// Deterministic output: encoding/json sorts map keys when marshalling a map, so the file is
	// diff-stable across runs without an explicit sort here.
	data, err := json.MarshalIndent(branchLinkFile{Version: branchLinkFileVersion, Links: links}, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	// Containment before write, same contract as SaveBaseline: the destination is derived from
	// cfg.RoadmapDir, which comes from trackfw.yaml (user-supplied), so it must be guarded.
	// Fail closed: without a verifiable root we refuse instead of writing.
	target := BranchLinkPath(cfg)
	cwd, cwdErr := getwdFn()
	if cwdErr != nil {
		return pathguard.RefuseUnverifiableRoot(target, cwdErr)
	}
	root := cwd
	if resolved, resolveErr := filepath.EvalSymlinks(cwd); resolveErr == nil {
		root = resolved
	}
	abs := target
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, target)
	}
	// No MkdirAll here on purpose: pathguard.GuardedWrite creates the parent directory ITSELF,
	// after its own containment check. Adding a second one would create the directory BEFORE the
	// guard fires — the exact ordering defect the guard exists to prevent — and would add a write
	// primitive site to check-write-containment.sh for nothing.
	return pathguard.GuardedWrite(root, abs, data, 0o644)
}

// branchSlugOf returns the slug part of "type/slug", or the whole name when there is no prefix.
func branchSlugOf(branch string) string {
	if i := strings.Index(branch, "/"); i >= 0 {
		return branch[i+1:]
	}
	return branch
}

// BranchRoadmapResolution is the outcome of the full D1 resolution order: written link first, name
// inference as fallback.
type BranchRoadmapResolution struct {
	Matched    bool
	Source     string // "written-link" | "inference" | "none"
	Roadmap    string // resolved roadmap filename, when exactly one is identified
	Candidates []string
	Warnings   []string
}

// ResolveBranchRoadmap answers "is this branch governed, and by which roadmap?" using the full D2
// resolution order (written link → stale link warn → wip∪blocked inference → done/ only if absent
// from origin/main base).
//
// It delegates to ResolveBranchRoadmapForExisting, resolving blocked/ dirs from cfg internally.
// Callers in branch_roadmap_match_ml3a_test.go pass their own wip/done dir lists; those are
// forwarded unchanged, and blocked/ is added from cfg (typically empty in test fixtures).
func ResolveBranchRoadmap(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) BranchRoadmapResolution {
	return ResolveBranchRoadmapForExisting(cfg, branch, wipDirs, resolveBlockedDirs(cfg), doneDirs)
}

// ResolveBranchRoadmapForExisting answers "is this EXISTING branch governed?" for the
// validate/commit/push/ship gates (D2 of ADR-2026-10-01). It is the single source of truth for
// branch governance on existing branches; validateBranchHasWIPRoadmap delegates here.
//
// Resolution order:
//
//  1. WRITTEN LINK — target in wip/ ∪ blocked/ ∪ done/ (BranchLinkFor scope, D4) → governs.
//  2. STALE LINK — target left scope → emit BranchLinkStaleWarning, fall through to inference.
//  3. INFERENCE in wip/ ∪ blocked/ → governs (D2 item 2).
//  4. INFERENCE in done/ → governs only when the roadmap is ABSENT from done/ in the base tree
//     (deriveOriginDefaultBranch). "Absent from base" means "moved by this branch" (D2 item 3,
//     Definition of Done path).
//     D3: if the base ref is not resolvable, or git ls-tree fails for any done/ dir → accept +
//     emit branch_done_scope_unverifiable warning. Never a violation. Never silent.
//
// 🔴 Must not touch MatchRoadmapsForBranchSlug, branchRoadmapTokens, roadmapContentSlug,
// sharedTokenCount or the two constants (D6 of ADR-2026-10-01).
func ResolveBranchRoadmapForExisting(cfg config.ProjectConfig, branch string, wipDirs, blockedDirs, doneDirs []string) BranchRoadmapResolution {
	res := BranchRoadmapResolution{Source: "none"}

	// Step 1 + 2: written link (BranchLinkFor internally checks wip∪blocked∪done, D4).
	link := BranchLinkFor(cfg, branch, wipDirs, doneDirs)
	if link.InScope {
		res.Matched = true
		res.Source = "written-link"
		res.Roadmap = link.Roadmap
		return res
	}
	if link.Present {
		// Stale: emit warning, fall through to inference.
		res.Warnings = append(res.Warnings, BranchLinkStaleWarning(cfg, branch, link.Roadmap))
	}

	slug := NormalizeBranchSlug(branchSlugOf(branch))

	// Build the combined candidates list (wip∪blocked∪done) for diagnostics.
	allDirs := append(append(append([]string{}, wipDirs...), blockedDirs...), doneDirs...)
	_, allCandidates := MatchRoadmapsForBranchSlug(slug, allDirs, nil)
	res.Candidates = allCandidates

	// Step 3: inference in wip∪blocked.
	wipBlockedDirs := append(append([]string{}, wipDirs...), blockedDirs...)
	wipBlockedMatches, _ := MatchRoadmapsForBranchSlug(slug, wipBlockedDirs, nil)
	if len(wipBlockedMatches) > 0 {
		res.Matched = true
		res.Source = "inference"
		if len(wipBlockedMatches) == 1 {
			res.Roadmap = wipBlockedMatches[0]
		}
		return res
	}

	// Step 4: inference in done/ — only if roadmap was moved by this branch (absent in base).
	doneMatches, _ := MatchRoadmapsForBranchSlug(slug, nil, doneDirs)
	if len(doneMatches) == 0 {
		return res // no matches anywhere
	}

	ref, refOK := deriveOriginDefaultBranch()
	if !refOK {
		// D3: no resolvable origin ref — accept + warn.
		res.Matched = true
		res.Source = "inference"
		if len(doneMatches) == 1 {
			res.Roadmap = doneMatches[0]
		}
		res.Warnings = append(res.Warnings,
			"branch_done_scope_unverifiable: cannot verify done/ scope — no resolvable origin ref; accepting match in done/ with degraded confidence")
		return res
	}

	// For each done match, determine whether it is absent from all done/ dirs in the base tree.
	// Absent → moved by this branch → governs. Present in any → already done before this branch.
	var movedByBranch []string
	for _, match := range doneMatches {
		inBase := false
		lsTreeFailed := false
		for _, doneDir := range doneDirs {
			baseSet, err := mdBasenamesInGitTreeWithError(ref, doneDir)
			if err != nil {
				// D3: ls-tree failed for this dir — accept + warn (never deadlock).
				lsTreeFailed = true
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"branch_done_scope_unverifiable: git ls-tree %s -- %s failed: %v; accepting match in done/ with degraded confidence",
					ref, doneDir, err))
				break
			}
			if baseSet[match] {
				inBase = true
				break
			}
		}
		if lsTreeFailed || !inBase {
			movedByBranch = append(movedByBranch, match)
		}
	}

	if len(movedByBranch) > 0 {
		res.Matched = true
		res.Source = "inference"
		if len(movedByBranch) == 1 {
			res.Roadmap = movedByBranch[0]
		}
	}
	return res
}
