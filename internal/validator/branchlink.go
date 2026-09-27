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
	// InScope is true when the recorded roadmap is still a candidate in wip/ or done/.
	// Present && !InScope is a STALE link — the roadmap was renamed, moved out of wip/+done/ or
	// deleted. The ADR forbids answering a stale link with a silent fallback.
	InScope bool
}

// BranchLinkFor resolves the written link for branch against the roadmaps currently in
// wipDirs+doneDirs. It is the single reader of the link file — `validate`, `commit` and `ship` all
// go through here.
func BranchLinkFor(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) BranchLinkStatus {
	recorded := strings.TrimSpace(readBranchLinks(cfg)[branch])
	if recorded == "" {
		return BranchLinkStatus{}
	}
	status := BranchLinkStatus{Roadmap: recorded, Present: true}
	for _, dir := range append(append([]string{}, wipDirs...), doneDirs...) {
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
// wip/ nor done/. It is a WARNING and never a violation, by decision: promoting it would break the
// additive order of D4 — a branch that passes today by inference must not start failing because a
// stale accelerator entry exists next to it. Silence is what the ADR forbids, not leniency.
func BranchLinkStaleWarning(cfg config.ProjectConfig, branch, roadmap string) string {
	return fmt.Sprintf(
		"branch_link_stale: the written link for branch %q names roadmap %q, which is no longer in wip/ nor done/ — falling back to name inference. Re-create the link with 'trackfw branch new', or drop the entry from %s",
		branch, roadmap, BranchLinkPath(cfg),
	)
}

// RecordBranchLink writes the branch↔roadmap link for branch, resolving the roadmap by inference at
// the moment of creation — which is the moment the information is still exact.
//
// It records ONLY when the inference identifies exactly ONE roadmap. With two or more, there is no
// single truth to write, and inventing one by scan order is the very defect ML-1C removed from
// findRoadmap. With none, `branch new` has already blocked and never reaches here.
func RecordBranchLink(cfg config.ProjectConfig, branch string) error {
	slug := NormalizeBranchSlug(branchSlugOf(branch))
	wipDirs := ResolveWIPDirs(cfg)
	doneDirs := ResolveDoneDirs(cfg)
	matches, _ := MatchRoadmapsForBranchSlug(slug, wipDirs, doneDirs)
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

// ResolveBranchRoadmap answers "is this branch governed, and by which roadmap?" in the order D1
// decided:
//
//  1. WRITTEN LINK — if a link is recorded and its target is still in wip/ or done/, that is the
//     answer. No inference runs.
//  2. STALE LINK — recorded but the target left wip/+done/: fall back to inference AND emit
//     BranchLinkStaleWarning. 🔴 The ADR names silent fallback as the one answer not allowed here.
//  3. INFERENCE — MatchRoadmapsForBranchSlug (substring ∪ token overlap).
func ResolveBranchRoadmap(cfg config.ProjectConfig, branch string, wipDirs, doneDirs []string) BranchRoadmapResolution {
	slug := NormalizeBranchSlug(branchSlugOf(branch))
	matches, candidates := MatchRoadmapsForBranchSlug(slug, wipDirs, doneDirs)

	res := BranchRoadmapResolution{Candidates: candidates, Source: "none"}

	link := BranchLinkFor(cfg, branch, wipDirs, doneDirs)
	switch {
	case link.InScope:
		res.Matched = true
		res.Source = "written-link"
		res.Roadmap = link.Roadmap
		return res
	case link.Present:
		res.Warnings = append(res.Warnings, BranchLinkStaleWarning(cfg, branch, link.Roadmap))
	}

	if len(matches) > 0 {
		res.Matched = true
		res.Source = "inference"
		if len(matches) == 1 {
			res.Roadmap = matches[0]
		}
	}
	return res
}
