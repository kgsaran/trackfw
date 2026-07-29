'use strict'

// Unit tests for npm/src/lib/update-engine.js's tildeify — ML-6H
// (ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador),
// docs/cli-parity.md "Declared harness targets — pinned list": path is
// rendered tilde-abbreviated, never absolute.

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')

const { tildeify } = require('../src/lib/update-engine')

test('tildeify abbreviates a plain absolute path under home', () => {
  const home = '/tmp/trackfw-home'
  const abs = path.join(home, '.claude', 'skills', 'trackfw')
  assert.equal(tildeify(home, abs), '~/.claude/skills/trackfw')
})

test('tildeify returns "~" for the home directory itself', () => {
  const home = '/tmp/trackfw-home'
  assert.equal(tildeify(home, home), '~')
})

// Regression: macOS's default $TMPDIR already ends in a separator, so a
// test harness (or any caller) building HOME as `path.join($TMPDIR, 'foo')`
// can still end up with a HOME string ending in "/" after path.join
// collapses internal doubles — and path.normalize PRESERVES a trailing
// separator when the input already has one, rather than stripping it. The
// pre-fix code appended another path.sep unconditionally before the
// prefix check, comparing against ".../foo//", which the (separator-free)
// absPath never matched — silently falling through to the absolute-path
// branch instead of abbreviating.
test('tildeify abbreviates correctly when HOME contains a double separator and/or a trailing separator', () => {
  const home = '/tmp/foo//bar/'
  const abs = path.join(home, '.claude', 'skills', 'trackfw')
  assert.equal(tildeify(home, abs), '~/.claude/skills/trackfw')
})

test('tildeify treats a home with a trailing separator the same as one without', () => {
  const abs = '/tmp/foo/bar/.claude/skills/trackfw'
  assert.equal(tildeify('/tmp/foo/bar/', abs), tildeify('/tmp/foo/bar', abs))
  assert.equal(tildeify('/tmp/foo/bar/', abs), '~/.claude/skills/trackfw')
})

test('tildeify returns the normalized absolute path unabbreviated when outside home', () => {
  assert.equal(tildeify('/tmp/foo/bar', '/somewhere/else'), '/somewhere/else')
})
