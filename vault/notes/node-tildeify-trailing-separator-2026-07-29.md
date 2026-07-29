---
title: "node-tildeify-trailing-separator"
tags: [update-harness, update-project, json-parity, node, cross-runtime, ML-6H, tildeify]
date: 2026-07-29
related: [node-tildeify-double-slash-home-2026-07-29]
---

# node-tildeify-trailing-separator

## Problem

After the ML-6F/ML-6G fix to `npm/src/lib/update-engine.js:tildeify` (see
[[node-tildeify-double-slash-home-2026-07-29]]), which added `path.normalize()` on both `homeRoot` and
`absPath` before comparing, a narrower residual case still defeated tilde-abbreviation: when
`homeRoot` (i.e. `$HOME`) **itself already ends in a path separator**, `tildeify` still fell through
to the raw absolute path instead of `~/...`.

## Root cause

```js
function tildeify(homeRoot, absPath) {
  const normalizedHome = path.normalize(homeRoot)
  const normalizedPath = path.normalize(absPath)
  if (normalizedPath === normalizedHome) return '~'
  if (normalizedPath.startsWith(normalizedHome + path.sep)) return '~' + normalizedPath.slice(normalizedHome.length)
  return normalizedPath
}
```

`path.normalize` collapses an **internal** doubled separator (`"a//b"` → `"a/b"`), but it
**preserves a trailing separator** if the input already has one — `path.normalize('/tmp/foo//bar/')`
is `'/tmp/foo/bar/'`, not `'/tmp/foo/bar'`. `normalizedPath` (built via `path.join`, which never
leaves a trailing separator) never carries one. So when `homeRoot` itself ends in `/` — which happens
on macOS whenever a caller builds `HOME` as `path.join($TMPDIR, 'something')`, because macOS's default
`$TMPDIR` already ends in `/` and `mktemp -d "$TMPDIR/x"`-style shell concatenation reproduces the same
shape — the prefix check compares `normalizedPath.startsWith(normalizedHome + path.sep)`, i.e.
`"...bar/.claude/...".startsWith("...bar//")`, which is **false** (single vs. double separator), and
the function falls through to the unabbreviated absolute path.

Reproduction:

```js
node -e "
const {tildeify} = require('./npm/src/lib/update-engine');
const path = require('path');
const home = '/tmp/foo//bar/';
const abs = path.join(home, '.claude', 'skills', 'trackfw');
console.log(tildeify(home, abs)); // '/tmp/foo/bar/.claude/skills/trackfw' — should be '~/.claude/skills/trackfw'
"
```

This surfaced during ML-6H (`ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador`),
while proving `trackfw update`'s project-scope target list/state parity — `scripts/check-update-parity.sh`
builds every `HOME` under `$WORK` (itself under `${TMPDIR:-/tmp}`), so any scenario whose `$WORK` path
composition happens to leave a trailing separator on the constructed `HOME` string hits this.

## Fix

Strip a trailing `path.sep` from `normalizedHome` (root `"/"` excepted, to avoid producing an empty
string and mis-tildeifying everything) before the prefix check:

```js
function tildeify(homeRoot, absPath) {
  let normalizedHome = path.normalize(homeRoot)
  if (normalizedHome.length > path.sep.length && normalizedHome.endsWith(path.sep)) {
    normalizedHome = normalizedHome.slice(0, -path.sep.length)
  }
  const normalizedPath = path.normalize(absPath)
  if (normalizedPath === normalizedHome) return '~'
  if (normalizedPath.startsWith(normalizedHome + path.sep)) return '~' + normalizedPath.slice(normalizedHome.length)
  return normalizedPath
}
```

Covered by `npm/tests/update-engine.test.js` (new file): plain abbreviation, home-equals-path,
doubled internal separator, trailing separator, and the not-under-home fallback.
