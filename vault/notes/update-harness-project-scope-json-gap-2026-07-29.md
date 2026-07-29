---
title: "update-harness-project-scope-json-gap"
tags: [go, cli, update, parity, contract, ambiguity]
date: 2026-07-29
related: [barrier-gates-check-key-order-divergence-go-2026-07-29]
---

# update-harness-project-scope-json-gap

## Problem

`docs/cli-parity.md` ("## `trackfw update` vs `trackfw update harness`") states, in its
own normative table, that the four flags (`--dry-run`, `--json`, `--targets`,
`--install-missing`) and the four-state contract apply to **both** `trackfw update`
(project scope) and `trackfw update harness` (global scope) — the "Applies to" column
literally says "both" for every flag row, and the prose says "Both commands report one
state per target."

ML-6B (`docs/roadmaps/wip/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`)
only implemented the full contract (states, flags, JSON document) for
`trackfw update harness`. `trackfw update` (project scope) kept its pre-existing
plain-text, flag-less behavior — it only lost the one call that mutated global state
(`ForceInstallSkills()`, i.e. the legacy `~/.claude/skills/trackfw/SKILL.md` write).

## Root cause of the gap (not a bug — a scope decision)

Two things pointed in different directions:

1. `docs/cli-parity.md`'s own table says the flags/states/JSON apply to project scope too.
2. ML-6B's *concrete* acceptance criteria only test harness behavior (no project scope
   `--json`/`--dry-run` case is asserted anywhere in the ML).

Building full state detection for the six heterogeneous project-scope steps (agent
rules injection across N detected files, attention hooks, validate script, CI workflow,
surgical git hooks, `.claude/commands/trackfw`) requires either:
- true content diffing before/after each step (feasible, but most of these functions
  don't expose a "compute desired content without writing" helper today), or
- a real `--dry-run` code path threaded through every one of those generators.

Doing this unilaterally in Go — inventing a project-scope target id list no one else
has seen — risks exactly the kind of cross-runtime divergence
[[barrier-gates-check-key-order-divergence-go-2026-07-29]] describes: Node.js (ML-6C)
and Python (ML-6D) would have to blindly guess or reverse-engineer the same
granularity from the Go source, with no shared authoring point.

## Decision made

Implemented the full contract (four states, four flags, pinned JSON key order) only
for `trackfw update harness`, where the contract's own JSON example lives
(`"codex-agents"`/`"claude-skill"`, `~/.codex/agents` style paths). `trackfw update`
(project) was left exactly as before, minus the one global-mutation call.

## What the next agent needs to know

If ML-6C/6D (Node.js/Python) are asked to "mirror the Go implementation," they will
only find harness-side flags/JSON in the Go code — there is no Go precedent for
project-scope `--json`/`--dry-run`/`--targets`. Before adding it to any one runtime,
get the target-id list and state-detection strategy agreed across all three at once
(likely a dedicated ML), not invented independently per runtime.
