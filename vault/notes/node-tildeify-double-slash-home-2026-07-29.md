---
title: "node-tildeify-double-slash-home"
tags: [update-harness, json-parity, node, cross-runtime, ML-6G, mktemp, tmpdir]
date: 2026-07-29
related: [update-harness-project-scope-json-gap-2026-07-29]
---

# node-tildeify-double-slash-home

## Problem

`trackfw update harness --json` in the Node.js CLI fails to tilde-abbreviate a target's `path`
(`docs/cli-parity.md` pins `path` as always tilde-abbreviated, e.g. `~/.claude/skills/trackfw/SKILL.md`,
never absolute) when `$HOME` contains a doubled path separator (`//`) anywhere in it — it silently
falls back to the raw absolute path instead. This is exactly the class of divergence
`scripts/check-update-parity.sh` (ML-6G) was created to catch, and it surfaced on the very first run
of that gate, over a completely ordinary fixture.

## Root cause

`npm/src/lib/update-engine.js:tildeify`:

```js
function tildeify(homeRoot, absPath) {
  if (absPath === homeRoot) return '~'
  if (absPath.startsWith(homeRoot + path.sep)) return '~' + absPath.slice(homeRoot.length)
  return absPath
}
```

`absPath` is always built via `path.join(homeRoot, ...)` elsewhere (e.g.
`npm/src/commands/update-harness.js:claudeSkillTarget`), and `path.join` **normalizes** its result —
collapsing any doubled slash in `homeRoot` down to one. `tildeify`'s own `startsWith` check compares
against the **raw, un-normalized** `homeRoot` string. If `homeRoot` (i.e. `os.homedir()`, i.e. the
literal `$HOME` env var — Node's `os.homedir()` does not resolve or normalize it) contains `//`
anywhere, the normalized `absPath` can never start with `homeRoot + path.sep` and the function falls
through to `return absPath` (the raw absolute path) unconditionally:

```js
node -e "
const path=require('path');
const homeRoot='/var/folders/x/T//abc';
const filePath=path.join(homeRoot,'.claude','skills','trackfw','SKILL.md');
console.log(filePath.startsWith(homeRoot+path.sep)); // false
"
```

## Why this is easy to miss on a real dev machine, and why the gate hit it immediately

A developer's real `$HOME` (e.g. `/Users/alice`) essentially never contains a doubled slash, so this
never reproduces in ad hoc manual testing. But `scripts/check-update-parity.sh` — like several
existing gates (`check-rules-parity.sh`, `check-slash-parity.sh`, `check-barrier.sh`) — creates its
fixture directories with:

```bash
WORK=$(mktemp -d "${TMPDIR:-/tmp}/trackfw-update-parity.XXXXXX")
```

On macOS, `$TMPDIR` is set by the OS to something like
`/var/folders/js/<hash>/T/` **with a trailing slash**. Concatenating `"${TMPDIR:-/tmp}/trackfw-..."`
onto a `TMPDIR` that already ends in `/` produces a directory name containing `//`, which then
propagates into every `$HOME` this gate sets (`HOME="$WORK/s1-home-node"` etc.) — and Node's
`tildeify` breaks on exactly that shape of string. This is the **first** gate that exercises path
tilde-rendering, so no earlier gate had a reason to notice this despite using the identical
`mktemp -d "${TMPDIR:-/tmp}/..."` pattern for years.

## Confirmed scope

- Reproduces with plain `node`, no trackfw code beyond `tildeify` involved (see snippet above).
- Only Node.js is affected. Go and Python render tilde-abbreviated paths correctly in the same
  fixture (verified via `scripts/check-update-parity.sh` scenario 1, `go-vs-python` comparison
  passes; only `go-vs-node` fails, isolated to the `path` field).
- Affects every harness target's `path`, not just `claude-skill` — any target built via
  `path.join(homeRoot, ...)` + `tildeify(homeRoot, ...)` in
  `npm/src/commands/update-harness.js`/`npm/src/lib/update-engine.js`.

## Fix (not applied by this ML — out of scope for QA/scripts)

`tildeify` should normalize `homeRoot` (e.g. `path.normalize` or `path.resolve`) before both the
equality and `startsWith` checks, so a doubled/trailing separator in `$HOME` can never defeat
abbreviation. This touches `npm/src/lib/update-engine.js`, a runtime file outside QA's
`scripts/`+`Makefile` scope for ML-6G (the same specialist track already active on ML-6F should pick
this up, since it is the same "path rendering" divergence class ML-6F is already correcting, just a
second trigger condition for it).

No workaround was applied inside `scripts/check-update-parity.sh` to dodge this (e.g. stripping
`TMPDIR`'s trailing slash before `mktemp`) — doing so would have masked the real product defect
that the gate exists to surface, which is the opposite of this gate's purpose.
