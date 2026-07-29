---
title: "commander-nested-subcommand-duplicate-flag-drops-parent"
tags: [nodejs, commander, cli-parsing, update-harness, ML-6C]
date: 2026-07-29
related: [barrier-gates-check-key-order-divergence-go-2026-07-29, update-harness-project-scope-json-gap-2026-07-29]
---

# commander-nested-subcommand-duplicate-flag-drops-parent

## Problem

Building `trackfw update harness` (ML-6C, ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador)
as a real `commander.Command` nested under `update` via `update.addCommand(harnessCmd)` silently
drops every option on invocation, **specifically and only** when the child command redeclares an
option with the same long flag name (`--json`, `--dry-run`, `--targets`, `--install-missing`) as
the parent. `trackfw update harness --json` parses successfully (no error, no usage message) but
the child action receives `{}` — the value binds to the ANCESTOR's `opts()` instead
(`update.opts()` shows `{ json: true }`, `harness.opts()` shows `{}`).

commander@12.0.0, reproduced in isolation:

```js
const update = new Command('update')
const harness = new Command('harness')
harness.option('--json')
harness.action((opts) => console.log(opts))   // logs {}
update.addCommand(harness)
update.option('--json')
update.action((opts) => console.log(opts))    // logs { json: true } — the '--json' from
                                               // 'update harness --json' landed here
program.addCommand(update)
program.parseAsync(['node', 'x', 'update', 'harness', '--json'])
```

Renaming either flag (e.g. parent `--foo`, child `--bar`) makes it work correctly. Removing the
parent's own `.option()`/`.action()` (making `update` a pure dispatcher with no leaf behavior of
its own) also makes it work correctly. The bug only manifests when a `Command` is simultaneously
(a) a leaf with its own action and options, and (b) a parent of a subcommand that redeclares an
identical flag name.

## Why this matters for `trackfw update` / `trackfw update harness`

The frozen contract (`docs/cli-parity.md`, "`trackfw update` vs `trackfw update harness`")
requires **the same four flags** (`--dry-run`, `--json`, `--targets`, `--install-missing`) on
both commands. That is exactly the triggering condition for this commander quirk — there was no
way to satisfy the contract with a naive parent+child-Command structure.

## Fix

`npm/src/commands/update.js` is a single `commander.Command` with an optional positional
argument (`cmd.argument('[mode]')`) instead of a nested `Command`:

```js
cmd.argument('[mode]')
cmd.option('--dry-run') // ...and the other three, declared once
cmd.action((mode, options) => {
  if (mode === 'harness') return require('./update-harness').run(options)
  if (mode) { /* usage error */ }
  // ...project-scoped update using the same `options`
})
```

`npm/src/commands/update-harness.js` exports a plain `run(options)` function, not its own
`Command` — there is exactly one `Option` object per flag name in the whole tree, so there is
nothing for commander to misattribute.

## How to detect this in future work

If a nested subcommand's action logs `{}` (or defaults) for a flag you can see was passed on the
command line, and the *parent* command also declares that flag, suspect this. Confirm with
`command.opts()` printed from inside the child action vs. the parent's `opts()` after
`parseAsync` resolves — if the value shows up on the parent instead, this is the same bug.
Renaming one of the two flags is the fastest isolation check.
