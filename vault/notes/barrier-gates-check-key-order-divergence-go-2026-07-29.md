---
title: "barrier-gates-check-key-order-divergence-go"
tags: [barrier, json-parity, cli-parity, go, cross-runtime, ML-4A]
date: 2026-07-29
related: [barrier-contract-xfail-false-positive-2026-07-29]
---

# barrier-gates-check-key-order-divergence-go

## Problem

`trackfw barrier <roadmap> --wave <n> --json` is contractually required (`docs/cli-parity.md` →
`## trackfw barrier` → "Determinism contract": *"Key order is fixed as shown"*) to emit the `gates`
check with key order `name, status, commands, evidence, failures` — exactly as pinned in the JSON
example in that section. Node.js (`npm/src/commands/barrier.js`) and Python
(`pypi/trackfw/commands/barrier.py`) both emit that order. **The Go implementation
(`internal/commands/barrier.go`) does not**: it emits `name, status, evidence, failures, commands`.

Root cause: the `barrierCheck` struct declares fields in the order `Name, Status, Evidence,
Failures, Commands` (with `Commands *[]string` tagged `omitempty`). `encoding/json` marshals
struct fields in declaration order regardless of tag order, so the `commands` key always lands
last for the `gates` check, in every response — passed or blocked, with or without gate commands.

## Why the existing test suites don't catch it

- `internal/commands/barrier_contract_test.go` (`TestBarrierContract_JSONDeterministico`) and the
  Node/Python twins unmarshal the JSON into a typed struct/object before asserting — Go's
  `encoding/json.Unmarshal`, Node's `JSON.parse`, and Python's `json.loads` are all key-order
  agnostic on the read side, so a struct-based assertion can never observe this drift.
- ML-2D (the prior corrective wave for cross-runtime divergence) fixed two *string content*
  mismatches (the "no ML found" message and the two exit-2 messages) but did not add a raw-text,
  order-preserving JSON comparison across the three runtimes — so this class of defect (shape/order
  correct in the contract's own example, wrong in the shipped Go binary) had zero coverage.

## How it was found

`scripts/check-barrier.sh` (ML-4A, scenario 6 — "three runtimes agree byte-for-byte over the same
fixture") reparses each runtime's `--json` output with `json.loads` + `json.dump(..., indent=2)`
**without `sort_keys`**, so Python's dict-preserves-insertion-order behavior surfaces the exact
order each runtime emitted, while normalizing away irrelevant whitespace differences (Node
pretty-prints with `indent=2`, Go/Python emit compact JSON). Diffing Go's output against Node's
(or Python's) over an identical fixture with a declared gate reproduces the divergence directly:

```diff
      "name": "gates",
      "status": "passed",
+     "commands": [
+       "true"
+     ],
      "evidence": [
        "true: exit 0"
      ],
-     "failures": [],
-     "commands": [
-       "true"
-     ]
+     "failures": []
```

## Falsification

Reproduced manually against a single fixture (`docs/req`, `docs/roadmaps/wip`, one wave, one ML,
one `**Gates da wave:**` block with `true`):

```bash
cd <fixture> && "$GO_BIN" barrier ROADMAP-x --wave 1 --json      # commands last
cd <fixture> && node npm/bin/trackfw barrier ROADMAP-x --wave 1 --json   # commands right after status
cd <fixture> && PYTHONPATH=pypi python3 -m trackfw barrier ROADMAP-x --wave 1 --json  # commands right after status
```

Exit-2 messages, the `mls_complete`/`acceptance_evidence`/`validate` checks, and the wave-with-zero-MLs
message were all re-verified as byte-identical across the three runtimes — only the `gates` check's
key order diverges, and only in the Go runtime.

## Fix (not applied by this ML — out of scope)

`internal/commands/barrier.go`'s `barrierCheck` struct field order should become
`Name, Status, Commands, Evidence, Failures` to match Node/Python and the pinned contract. This
touches `internal/commands/barrier.go`, which is an ML-2A file, not an ML-4A file (`scripts/`,
`README.md`, `site/guide/`) — QA does not own that file under this handoff's scope.

`scripts/check-barrier.sh` IS wired into the `parity` Makefile target (ML-4A ENTREGÁVEL 3) despite
this open defect — a gate that isn't wired can't be enforced, and it's exactly the "gate exists but
proves nothing" failure mode this roadmap exists to prevent. `make quality` is therefore **red**
until a corrective microbatch (analogous to ML-2D — call it ML-2E) lands the one-line struct
reorder above; it clears the moment that lands, with no further change to the gate itself.

Confirmed scope of the defect: only the `--json` output is affected. The text report
(`trackfw barrier <roadmap> --wave <n>`, no `--json`) is not pinned by `docs/cli-parity.md` at all
(only the JSON document has a "Determinism contract"), and the three runtimes already render
completely different text layouts by design — so there is nothing to fix there.
