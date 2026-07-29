---
title: "barrier-contract-xfail-false-positive"
tags: [barrier, pytest, xfail, argparse, cli-parity, pending-tests]
date: 2026-07-29
related: [barrier-git-authority-subagents-2026-07-29]
---

# barrier-contract-xfail-false-positive

## Problem

The ML-1A contract tests for `trackfw barrier` (`internal/commands/barrier_contract_test.go`,
`npm/tests/barrier-contract.test.js`, `pypi/tests/test_barrier_contract.py`) must fail *now* —
before `barrier` is implemented (ML-2A/2B/2C) — because they are marked pending
(`t.Skip` in Go, `{ skip: ... }` in Node, `@pytest.mark.xfail(strict=True)` in Python).

Go and Node fully skip the test body, so this is a non-issue for them: the assertions never run.
Python's `xfail(strict=True)` is different — it *executes* the body and expects it to raise. If the
body happens to pass anyway, pytest reports `XPASS(strict)` and the whole test run fails.

Scenario 7 (`roadmap_ou_wave_inexistente_e_erro_de_uso`) exposed exactly that trap: the Python CLI
(`pypi/trackfw/cli.py`, argparse-based) doesn't recognize `barrier` as a subcommand yet, so it
already exits with code 2 (argparse's own "invalid choice" usage error) and prints nothing that
looks like a `status: "blocked"` JSON document. Those two facts are precisely what the naive
scenario-7 assertions checked (`exit code == 2` and `no blocked JSON on stdout`) — so the test
accidentally **passed** before the barrier command existed, turning `xfail(strict=True)` into a
false green (XPASS failure at collection time).

## Root cause

Exit code 2 is argparse's generic "unknown command" exit code, and it happens to numerically
coincide with the contract's own "usage/resolution error" exit code (`docs/cli-parity.md` →
`## trackfw barrier` → Command surface table). The coincidence is purely numeric, not semantic:
argparse's error has nothing to do with wave/roadmap resolution.

## Solution / what to check when writing xfail(strict=True) pending tests

Don't just assert the exit code and the absence of a blocked-status document — those can pass by
accident when the command doesn't exist at all. Add at least one assertion that only a *real*
implementation of the contract can satisfy, e.g. that `stderr` explicitly names the thing that
failed to resolve (the wave number, or the roadmap basename). An argparse/commander/cobra "unknown
command" message never contains that content, so the pending test genuinely fails until ML-2A/2B/2C
land.

The same defensive assertion was added to the Go and Node versions of scenario 7 for semantic
parity across runtimes, even though those two don't currently need it (full skip never executes the
body) — it becomes load-bearing there too once the `t.Skip` / `{ skip }` marker is removed.

## Falsification

Verified locally: `PYTHONPATH=pypi python3 -m trackfw barrier ROADMAP-x --wave 99 --json` currently
prints argparse's generic `error: argument COMMAND: invalid choice: 'barrier' (choose from ...)` to
stderr and exits 2 — no mention of "wave", "99", or "roadmap" anywhere. Before the fix,
`pytest pypi/tests/test_barrier_contract.py` reported
`FAILED ... test_roadmap_ou_wave_inexistente_e_erro_de_uso — [XPASS(strict)]`. After adding the
stderr-content assertion, the same test reports `XFAIL` as expected, alongside the other 7 scenarios.
