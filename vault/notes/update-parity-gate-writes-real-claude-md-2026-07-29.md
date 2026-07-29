---
title: "update-parity-gate-writes-real-claude-md"
tags: [check-update-parity, claude-md, agents-install, scope-global, cross-runtime, ML-6H, gate-bug]
date: 2026-07-29
related: [node-tildeify-double-slash-home-2026-07-29, update-harness-project-scope-json-gap-2026-07-29]
---

# update-parity-gate-writes-real-claude-md

## Problem

Running `scripts/check-update-parity.sh` (directly, or transitively via `make quality`/`make parity`)
from the repository root modifies the **real** `CLAUDE.md` at the repo root, injecting the
`<!-- trackfw:rules:start -->...<!-- trackfw:rules:end -->` block. This happens silently — the gate
still reports every scenario `OK` and exits 0 — so `git status` after a green `make quality` is the
only way to notice it. This is exactly the same symptom a prior ML-6-era handoff already hit and had
to manually revert ("Um agente anterior fez isso e injetou o bloco trackfw:rules no CLAUDE.md do
próprio projeto"); this note pins down the actual mechanism so it stops being rediscovered by trial
and error.

## Root cause

`scripts/check-update-parity.sh`'s `install_claude_agents()` helper (used to seed Scenario 4, the
`--dry-run` harness proof):

```bash
install_claude_agents() {
  local home_dir=$1
  mkdir -p "$home_dir"
  HOME="$home_dir" "$GO_BIN" agents install --targets claude --scope global \
    --identity-preset neutral --json >/dev/null
}
```

Unlike every other invocation in this script (`run_update`, the `init` calls in Scenario 3), this one
is **not** wrapped in a `(cd "$some_scratch_dir" && ...)` subshell. `HOME` is redirected, but the
process's **working directory is never changed** — it inherits whatever `cwd` the script itself was
invoked from. When `check-update-parity.sh` is run from the repository root (directly, via `make
quality`, or via `make parity`), `agents install --scope global` runs with `cwd == repo root`.

`trackfw agents install`, even with `--scope global` (which correctly keeps the *catalog artifact*
writes under `$HOME`), independently detects and injects the agent-rules block into whatever
`CLAUDE.md` already exists in the **current working directory** — this rules-injection side effect is
not gated by `--scope` at all (scope only governs where the agents/skills catalog artifacts
themselves land, not the separate marker-delimited rules-block injection path shared with `trackfw
update`'s `agent-rules` target, `internal/generators/agentfiles.go:InjectRulesForTool`/
`InjectRulesDetected`). So `agents install --scope global` run from the repo root writes into the
repo's own `CLAUDE.md`.

Confirmed by direct reproduction (bisecting `make quality`'s script list one at a time, `git status
--short CLAUDE.md` after each):

```
check-cli-parity.sh ... check-rules-parity.sh  → all clean
check-update-parity.sh                          → CLAUDE.md dirty
```

And narrowed further inside that script to the `install_claude_agents` loop specifically (isolated by
copy-pasting the function + Scenario 4 setup into a standalone repro, checking `git status` between
the `install_claude_agents` loop and the `run_update ... harness --dry-run` calls that follow it —
the former alone already dirties `CLAUDE.md`; the latter, which IS a properly `cd`-wrapped
project-scope `update` call, does not add to it).

## Confirmed scope

- Every scenario inside `check-update-parity.sh` that uses `run_update()` (which always `cd`s into a
  scratch project dir first) is safe.
- Only `install_claude_agents()` (Scenario 4's fixture setup) is unsandboxed with respect to `cwd`.
- This is a pre-existing bug in the gate script itself, not in any of the three `trackfw` runtime
  implementations — `agents install --scope global`'s cwd-based rules injection is arguably correct
  runtime behavior (mirrors `update`'s `agent-rules` target, which is *supposed* to detect and inject
  into whatever agent config files exist in cwd); the gate script simply never isolates `cwd` for this
  one call.

## Fix (applied — ML-6I, `scripts/check-update-parity.sh`)

`install_claude_agents()` now runs inside a dedicated scratch cwd created under the script's own
`$WORK` (removed by the top-level `trap ... EXIT`), instead of the caller's inherited cwd:

```bash
install_claude_agents() {
  local home_dir=$1
  mkdir -p "$home_dir"
  local scratch_dir
  scratch_dir=$(mktemp -d "$WORK/agents-install-cwd.XXXXXX")
  (cd "$scratch_dir" && HOME="$home_dir" "$GO_BIN" agents install --targets claude --scope global \
    --identity-preset neutral --json >/dev/null)
}
```

Audited every other invocation in `check-update-parity.sh` and the sibling gates
(`check-barrier.sh`, `check-slash-parity.sh`, `check-rules-parity.sh`) for the same unsandboxed-cwd
pattern — none found; every other CLI call in those four scripts was already wrapped in
`(cd "$scratch_dir" && ...)`. `check-cli-parity.sh` invokes the CLIs without `cd`, but only for
read-only `--help`/`version`/`--version`, which have no side effects — left untouched (pre-existing,
already green, out of scope per the fix rationale below).

## Falsification-pipeline proof (ML-6I, `scripts/check-gates-falsify.sh` Scenario 18)

A green gate previously did **not** imply an untouched repo — the bug above passed every scenario
while still mutating `CLAUDE.md`. Scenario 18 (`falsify/no-repo-mutation`) closes that gap
mechanically: it snapshots `git status --porcelain` at `ROOT_DIR` before and after running
`check-update-parity.sh`, `check-barrier.sh`, `check-slash-parity.sh` and `check-rules-parity.sh`
(the four gates that invoke real CLI binaries from the repo root) and fails the pipeline if the
snapshots differ. This runs inside `make quality`/`make parity`, so "gate verde" now really does mean
"repositório intocado" without depending on a human/agent remembering to check manually.

## Verification note — `find -mmin`, not `-newermt`

The `find ~/.claude ... -newermt "-N hours"` form is **invalid** on this machine's `find` (BSD/`bfs`)
and fails silently, returning an empty result that looks like "nothing was written" even when it
would not otherwise have found anything anyway — a vacuous check. Use `find <dir> -mmin -N -type f`
instead when verifying that a real (non-redirected) `HOME` was not touched by a test run.
