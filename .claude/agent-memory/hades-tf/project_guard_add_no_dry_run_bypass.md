---
name: project_guard_add_no_dry_run_bypass
description: git add -An --no-dry-run passes guard but stages whole-tree; B-1 fix introduced last-flag-wins gap; B-3 blocker in Round 2 red-team
metadata:
  type: project
---

# Guard add: --no-dry-run negation bypass (B-3 blocker)

**Found:** 2026-10-17, Rodada 2 red-team REQ-2026-09-05.
**Status:** NOT FIXED — B-3 BLOQUEANTE in report.

## Fact

The B-1 fix (commit `ccccedc8`) decomposes combined short flags char-by-char. Char `n` in
`-An` sets `dryRun=true`, guard allows. Correct for genuine dry-run.

But git uses **last-flag-wins**: `--no-dry-run` and `--no-dry` (abbreviation) cancel `-n`.
Guard doesn't model this. `git add -An --no-dry-run` stages whole tree; guard passes it.

Measured in /tmp/tgit:
- `git add -An` → nothing staged (dry-run OK)
- `git add -An --no-dry-run` → both test files staged (whole-tree bypass)

**Why:** The B-1 fix added set-to-true but no last-wins negation.

**How to apply:** When reviewing guard fixes that add `dryRun=true`/`interactive=true`,
always test the corresponding `--no-*` negation forms. Fix: model negations as last-wins
(sequential overwrite), not OR-accumulation.

[[project_guard_add_pathspec_positional_bypass]] — same REQ, B-2 blocker Rodada 1
