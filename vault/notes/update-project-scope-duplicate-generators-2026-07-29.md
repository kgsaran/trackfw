---
title: "update-project-scope-duplicate-generators"
tags: [update-project, validate-script, agent-hooks, init, node, go, python, cross-runtime, ML-6H]
date: 2026-07-29
related: [update-harness-project-scope-json-gap-2026-07-29]
---

# update-project-scope-duplicate-generators

## Problem

`trackfw update` (project scope) reported `"state": "updated"` for `validate-script` and/or
`"state": "updated"` for `agent-hooks` against a project that had just been freshly `init`-ed by the
**same** runtime — i.e. running `update` immediately after `init` was not idempotent, even though no
external change had touched the file. This broke `scripts/check-update-parity.sh`'s Scenario 3
(go-vs-node, go-vs-python full JSON diff).

## Root cause 1 — Node.js had two different `scripts/trackfw-validate.sh` generators

`npm/src/generators/init.js:generateValidateScript(cfg)` (used by `trackfw init`, via `scaffold()`)
writes a rich, per-backend script (build-check lines vary with `cfg.backend`/`cfg.frontend`).
`npm/src/commands/discover.js:writeValidateScript(rootDir)` (used by `trackfw discover` and, until this
fix, by `trackfw update`'s `validate-script` target) writes a different, static 3-line script
unconditionally. Because `update`'s target called the **second** generator, every `update` run
overwrote the **first** generator's output with different bytes — even against an untouched project —
so `runFileTarget`'s content-hash diff correctly, but misleadingly, reported `"updated"`.

Fix: `update.js`'s `validate-script` target now calls the same `generators.generateValidateScript`
`init` uses (given a `cwd`-aware second parameter added for this — it previously assumed
`process.cwd()`, which is unsafe for `--dry-run`'s sandboxed apply). `discover.js`'s writer was left
as the intentionally simpler `discover`-only path; it is unrelated to `update`'s idempotency now that
`update` no longer calls it.

## Root cause 2 — Go's `init` never wrote agent-hooks; Node's and Python's did

`internal/generators/scaffold.go:Scaffold` never called `InjectHooksDetected` — only `trackfw update`
did. `npm/src/generators/init.js:scaffold` and `pypi/trackfw/generators/init_gen.py:scaffold` both
already called their runtime's `inject_hooks_detected`/`injectHooksDetected` as their last step. So a
freshly `init`-ed Go project had no `.claude/settings.json` (etc.) yet, and the first `update` run
correctly-but-divergently reported `"updated"` (a real, new file), while Node/Python — whose `init`
already installed it — reported `"skipped"` (already current) for the identical logical scenario.

This is a **pre-existing cross-runtime `init` parity gap** (paridade rule violation per the project's
global CLAUDE.md — a feature present in 2 of 3 runtimes), not a bug in `update`'s discriminator logic;
both runtimes were reporting correctly given their own `init`'s different starting state. Fixed by
porting the same `InjectHooksDetected(cwd)` call (non-fatal, warn-and-continue like `update`'s own
usage) into Go's `Scaffold`, as the last step, matching Node/Python's placement.

## Root cause 3 — Python's `init` never wrote `scripts/trackfw-validate.sh` at all

Python's `init` had **no** validate-script generator in its `init` path at all (only `discover.py`'s
private `_write_validate_script`, never called by `scaffold()`). Python's `update` also only declared
3 of the 5 pinned project-scope target ids (missing `validate-script` and `claude-commands` entirely —
see docs/cli-parity.md, "Declared project targets — pinned list"). Fixed by adding a single canonical
`generate_validate_script(cwd)` to `pypi/trackfw/generators/init_gen.py` (content is the simple,
backend-agnostic 3-line script — this runtime's `init` CLI has no `--backend`/`--frontend`/
`--pkg-manager` flags to select a richer per-backend script, a separate, pre-existing, intentionally
reduced Python `init` surface — adding those flags was out of scope for this ML), called from both
`scaffold()` and `update.py`'s new `validate-script` target, and `discover.py`'s private writer now
delegates to it instead of duplicating the template a third time.

## Ancillary bug this surfaced — Python's `--json` leaked human progress lines

Once `validate-script`/`claude-commands` were wired into Python's `update --json`, their generators'
`print()` calls leaked into stdout ahead of the JSON document, breaking `json.loads` on the output.
Go (`silenceStdout`) and Node (`silenceConsole`) already redirect stdout during target computation when
`--json` is set; Python had never needed this because its previous 3 targets
(`inject_rules_detected`/`inject_hooks_detected`) don't print on success. Fixed with a
`contextlib.redirect_stdout(os.devnull)`-based `_silence_stdout` context manager in
`pypi/trackfw/commands/update.py`, wrapping only the target-computation loop.

## Confirmed scope

All three runtimes agree byte-for-byte (key order preserved, no `sort_keys`) on `trackfw update --json`
and `trackfw update --json --dry-run` in a project each runtime just `init`-ed itself, per
`scripts/check-update-parity.sh` Scenario 3 (`update-project/json/three-runtimes-identical`).
