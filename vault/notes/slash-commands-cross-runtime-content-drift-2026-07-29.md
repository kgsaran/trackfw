---
title: "slash-commands-cross-runtime-content-drift"
tags: [slash-commands, cli-parity, cross-runtime, ML-5D, gate]
date: 2026-07-29
related: [barrier-gates-check-key-order-divergence-go-2026-07-29, barrier-contract-xfail-false-positive-2026-07-29]
---

# slash-commands-cross-runtime-content-drift

## Problem

`scripts/check-slash-parity.sh` (ML-5D,
ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador) proves the three CLI
runtimes install the exact same `.claude/commands/trackfw/*.md` slash commands, both in name
set and byte content. Wiring it into `make parity` immediately turned `make quality` red —
**not** because of a defect introduced by ML-5D, but because two pre-existing content
divergences already existed in HEAD across the three hand-maintained generators
(`internal/generators/scaffold.go`, `npm/src/generators/init.js`,
`pypi/trackfw/generators/init_gen.py`):

1. **`move.md` — "Estados válidos" list.** Go and Node.js list all six states
   (`backlog`, `analyzing`, `wip`, `blocked`, `done`, `abandoned`); Python's list omits
   `analyzing` entirely.
2. **`move.md` — example line.** Go and Python use `/trackfw:move meu-roadmap wip`; Node.js
   uses `/trackfw:move meu-roadmap analyzing`.
3. **`architect.md` — opening sentence.** Python's first line carries the extra
   `` (`/trackfw:architect`) `` parenthetical after "guia de arquitetura do trackfw"; Go and
   Node.js do not.

## Majority text per line (all three are clean 2-1 splits)

Each divergent line has an unambiguous 2-runtime majority — this is not an unresolvable tie:

| line | Go | Node.js | Python | majority | odd one out |
|---|---|---|---|---|---|
| `move.md` "Estados válidos" | has `analyzing` | has `analyzing` | missing `analyzing` | has `analyzing` | Python |
| `move.md` "Exemplo" | `wip` | `analyzing` | `wip` | `wip` | Node.js |
| `architect.md` opening sentence | no parenthetical | no parenthetical | `` (`/trackfw:architect`) `` parenthetical | no parenthetical | Python |

The recommended fix (ML-5F) is therefore mechanical, not a design decision:
1. `pypi/trackfw/generators/init_gen.py`: add `analyzing` to `move.md`'s states list.
2. `npm/src/generators/init.js`: change `move.md`'s example from
   `` `/trackfw:move meu-roadmap analyzing` `` to `` `/trackfw:move meu-roadmap wip` ``.
3. `pypi/trackfw/generators/init_gen.py`: drop the `` (`/trackfw:architect`) `` parenthetical
   from `architect.md`'s opening sentence.

**One open product-doc question, not a parity fact**: should the `Exemplo` line in `move.md`
demonstrate `wip` (2/3 majority today) or `analyzing` (arguably a more instructive example,
since `analyzing` is the state the roadmap-lifecycle rules ask agents to use as an intermediate
step before `wip`)? Either choice is a one-line edit applied identically to all three
generators — flagged here only because it is a judgment call about example content, not a
divergence needing arbitration.

## Falsification proof that the gate itself is non-vacuous

`scripts/check-gates-falsify.sh` scenario 14 corrupts `status.md`'s content in a throwaway copy
of the Node.js generator (a file that — as of this note — is identical across all three
runtimes) and asserts the gate reports `slash parity drift: status.md (go vs node)`. `status.md`
was deliberately chosen over `move.md`/`architect.md` specifically because those two already
drift on HEAD; corrupting an already-drifting file would not distinguish the injected defect
from the pre-existing noise.

## Gate design note — accumulate, don't fail-fast

`scripts/check-slash-parity.sh` accumulates every divergent file across both pairwise
comparisons (go-vs-node, go-vs-python) before exiting, following the `FAIL=1` pattern in
`check-artifact-parity.sh` rather than `check-barrier.sh`'s fail-fast style. A fail-fast gate
would have aborted on the first pre-existing `move.md` drift and never reached (or reported)
any additional corruption a future scenario might inject — the same principle already
documented for the Go `gates` check key-order gate landing "red but wired" in
`barrier-gates-check-key-order-divergence-go-2026-07-29`.

## Fix (not applied by ML-5D — out of scope)

Pick the canonical text for `move.md`'s states list and example line, and for `architect.md`'s
opening sentence, and land it identically in `internal/generators/scaffold.go`,
`npm/src/generators/init.js`, and `pypi/trackfw/generators/init_gen.py`. Recommended follow-up:
ML-5F. Neither ML-5B (`*/help*`, `root.go`, `commands/index.js`, `cli.py`,
`go_only_commands`) nor ML-5E (`agentfiles.go`, catalog install path) — the two agents running
in parallel with ML-5D — own any of these three generator files, so this is not parallel-agent
noise; it predates this wave.
