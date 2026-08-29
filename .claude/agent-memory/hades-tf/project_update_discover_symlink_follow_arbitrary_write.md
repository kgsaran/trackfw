---
name: project-update-discover-symlink-follow-arbitrary-write
description: trackfw update/discover write .github/workflows/trackfw-validate.yml following symlinks (no lstat) — arbitrary file overwrite/creation outside project, reproduced live in all 3 CLIs
metadata:
  type: project
---

Found 2026-08-28, REQ "gate de CI pinado na versão geradora e install.sh honrando TRACKFW_VERSION"
(barrier review). REPROVA'd for this finding (HIGH, not CRITICAL — written content is always the
fixed trackfw template, never attacker content).

**Mechanism:** presence checks (`os.Stat`/`fs.existsSync`/`os.path.isfile`) and the writes
(`os.WriteFile`/`fs.writeFileSync`/`open(...,'w')`) all follow symlinks by default, with no `lstat`
guard anywhere in `internal/generators/update.go`, `npm/src/commands/update.js`,
`pypi/trackfw/commands/update.py`, or the `discover.go`/`discover.js`/`discover.py` siblings.

- Live symlink at `.github/workflows/trackfw-validate.yml` pointing outside the project + plain
  `trackfw update` (even with `ci: none` in trackfw.yaml) → overwrites the symlink target with the
  trackfw CI-workflow template. Reproduced live in Go, Node, Python.
- Dangling symlink at the same path + `trackfw discover --init` → **creates** a new file at the
  attacker-chosen path outside the project (idempotent `isFile` guard returns false on a dangling
  link, so the writer proceeds). Reproduced live in Go; same code shape in Node/Python.
- `trackfw update --dry-run` is safe — its sandbox dereferences the symlink when copying the tree,
  so the write lands inside the throwaway sandbox, not at the real target. Verified live.

**Why this REQ specifically widened it:** `refreshDiscoverGitHubActionsWorkflowIfPresent` is a new
function this REQ added, and `ProjectTargetIDs`/`project_target_ids` now activates `ci-workflow`
whenever `discoverWorkflowPresent` is true — regardless of `cfg.CI`. A `ci: none` project (opted out
of CI writes) is newly in scope purely because a file/symlink exists at that path.

**Pre-existing sibling, not caused by this REQ:** the same pattern already existed for
`trackfw-gate.yml`/`.gitlab-ci-trackfw.yml` via `generateCIWorkflow` — reproduced live too. Treat as
a separate, broader "harden every update.go file writer against symlink-follow" REQ — do not fold
it into a narrow corrective ML.

**Fix shape:** `os.Lstat`/`fs.lstatSync(...,{throwIfNoEntry:false}).isSymbolicLink()`/
`os.path.islink` before deciding presence AND before writing — refuse (report divergence) rather
than follow, in both `update` and `discover`'s writers.

See [[feedback_verify_by_execution]] — this was found only because I built the binary and ran live
symlink PoCs in scratchpad dirs; reading the diff alone would have missed it (advisor caught the gap
after I'd only checked the *content* of the write, not the *path resolution*).
