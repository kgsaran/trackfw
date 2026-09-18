---
name: project_head_equals_disk_in_ci_committed_changes
description: In CI (pull_request event), HEAD IS the merge commit — HEAD==disk for all committed PR changes; credentialGuardAnchoredRules pattern closes only uncommitted edits; HEAD^1 unreachable in depth-1 clone (RC=128)
metadata:
  type: project
---

HEAD-vs-disk anchoring (credentialGuardAnchoredRules) is explicitly scoped to uncommitted edits per the code comment in validator.go:202-206. In CI with `actions/checkout@v7` (no `with:` block, default depth=1), `pull_request` events check out `refs/pull/N/merge` — the merge commit where HEAD already incorporates all PR changes. `git show HEAD:./trackfw.yaml` returns the merged version, byte-identical to disk. No differential detected.

**Measured:** PR branch commits `rules: {req_has_adr: off}`, merge commit is HEAD. `git show HEAD:./trackfw.yaml` == disk == merged version with rule off. HEAD-vs-disk comparison returns 0 difference → anchoring has no effect on committed PR changes.

**HEAD^1 unreachable in depth-1 clone (measured 2026-09-17):** `git rev-parse HEAD^1` → RC=128, `git show HEAD^1:trackfw.yaml` → RC=128. Shallow clone limits commit history — parents unreachable. Base-branch anchoring requires `fetch-depth: 2` or explicit `git fetch origin main` in workflow. Neither `trackfw-gate.yml` nor `trackfw-validate.yml` has this.

**Why:** The code comment is accurate: the pattern was designed to catch local uncommitted edits (someone editing trackfw.yaml to silence rules before running validate locally). It was never designed to catch committed PR changes.

**How to apply:** When reviewing any AC that claims "HEAD comparison detects severity downgrade via committed PR change," flag this: HEAD==disk in CI. The AC needs a workflow change (fetch-depth or explicit fetch) to be implementable. REQ #387 AC1 is affected — its stated mechanism (generalize credentialGuardAnchoredRules to all rules) closes uncommitted-edit channel only.
