'use strict'

// ROADMAP-2026-08-17 Wave 2/ML-2C -- hookArrayHasCommand/hasEntryPath must
// compare hook "command" paths after normalizing incidental formatting
// (double slashes, trailing slash), not as raw strings. Root cause: macOS's
// $TMPDIR ends in "/", so a $HOME built under it can contain "//" once
// concatenated with a literal path segment; the writer computes scriptPath
// via path.join (which normalizes), while a value written by hand or
// captured before normalization does not -- a byte-for-byte compare then
// silently fails to dedup. Mirrors internal/generators/
// guard_path_normalize_test.go (Go).

const test = require('node:test')
const assert = require('node:assert/strict')
const { normalizeGuardPath, samePathCommand } = require('../src/generators/hooks')

test('normalizeGuardPath collapses double slashes and strips trailing slash', () => {
  const cases = [
    ['', ''],
    ['/a/b', '/a/b'],
    ['//a/b', '/a/b'],
    ['/a//b', '/a/b'],
    ['/a/b/', '/a/b'],
    ['/a/b//', '/a/b'],
    ['//', '/'],
    ['/', '/'],
    ['/a/./b', '/a/./b'], // deliberately NOT resolved -- see hooks.js doc comment
    ['/a/b/../b', '/a/b/../b'], // deliberately NOT resolved
  ]
  for (const [input, want] of cases) {
    assert.equal(normalizeGuardPath(input), want, `normalizeGuardPath(${JSON.stringify(input)})`)
  }
})

test('samePathCommand tolerates the double-slash formatting produced by a trailing-slash $TMPDIR', () => {
  const a = '/var/folders/xx/yy/T//trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh'
  const b = '/var/folders/xx/yy/T/trackfw-falsify.ABC/s67-fake-home/.trackfw/scripts/trackfw-git-branch-guard.sh'
  assert.equal(samePathCommand(a, b), true)
})

// Non-regression half of risk #2 ("normalizar demais é perigoso"): two
// genuinely different scripts, or the same guard installed for two
// different users, must never compare equal.
test('samePathCommand does not match genuinely different paths', () => {
  const cases = [
    ['/home/alice/.trackfw/scripts/trackfw-git-branch-guard.sh', '/home/bob/.trackfw/scripts/trackfw-git-branch-guard.sh'],
    ['/a/b/trackfw-git-branch-guard.sh', '/a/bb/trackfw-git-branch-guard.sh'],
    ['/a/b/trackfw-git-branch-guard.sh', '/a/b/trackfw-credential-guard.sh'],
    ['/a/b', '/a/b/c'],
  ]
  for (const [a, b] of cases) {
    assert.equal(samePathCommand(a, b), false, `samePathCommand(${a}, ${b})`)
  }
})
