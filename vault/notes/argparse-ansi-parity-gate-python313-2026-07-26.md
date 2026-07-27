---
title: "argparse-ansi-parity-gate-python313"
tags: [ci, python, parity, ansi, argparse]
date: 2026-07-26
related: []
---

# argparse-ansi-parity-gate-python313

## Problem

`make quality` passes on CI (Python 3.10/3.12) but fails locally on Python 3.13+
with `python: missing command 'init'` (and similar for `adr`, `req`, `roadmap`,
`validate`, `status`, `log`, `configure`, `discover`, `update`, `metrics`, `sync`,
`context`, `baseline`, `help`, `plugins`, `serve`, `note`, `ship`).

The commands `agents` and `skills` were silently passing — not because the fix worked,
but because both words appear in a plain-text description line ("List and manage trackfw
agents") that has no ANSI decoration, so the word boundary grep happened to match
the wrong location.

## Root cause

Python 3.13 introduced ANSI colour output in argparse help by default. The check in
`scripts/check-cli-parity.sh` (and `assert_help_contract` in
`check-integration-cli-parity.sh`) used:

```bash
grep -Eq "(^|[[:space:]])${command}([[:space:]]|$)" <<<"$output"
```

When `init` is rendered as `ESC[1;32minit ESC[0m`, the character immediately before
`init` is `m` (the last byte of the ANSI code), not a space. The word-boundary regex
never matches, so the gate reports the command as absent.

`NO_COLOR=1` was not being set when invoking Python, so the runtime colourised its
output. The CI environment uses Python 3.10/3.12 which do not colourise by default,
hiding the failure.

## Solution

Two-layer fix applied in `fix(ci): gate de paridade imune a ajuda colorida do argparse`:

1. **Export `NO_COLOR=1` and `TERM=dumb`** at the top of `check-cli-parity.sh` so all
   child processes (including `check-integration-cli-parity.sh`, which is invoked as a
   subprocess) inherit the env. This prevents colour generation at the source.

2. **Strip ANSI sequences inline** inside `check_help()` and `assert_help_contract()`
   using `sed 's/\x1b\[[0-9;]*m//g'` before passing output to grep. This makes both
   functions immune even when a runtime ignores `NO_COLOR` (e.g. Node.js with
   `FORCE_COLOR` set, as observed during `make quality` output).

The `commands` array in `check-cli-parity.sh` was also extended to include `note` and
`ship`, which were verified present in all three runtime root help outputs.

Falsification verified: running the corrected `check_help` logic with a
`commands=(definitely-not-a-real-command)` override confirms exit=1 and the expected
"missing command" message, proving the gate is not vacuously passing.
