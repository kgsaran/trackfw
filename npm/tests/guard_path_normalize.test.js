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
const { normalizeGuardPath, samePathCommand, hasValidUNCPrefix } = require('../src/generators/hooks')

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

// ROADMAP-2026-09-03 ML-7B -- drive-letter anchored input gets "\"
// canonicalized to "/" (closing the seam ML-7A measured: path.win32.join
// always emits "\" on Windows, so a hand- or concat-built command with "/"
// never compared equal to the computed one), while UNC and every
// degenerate/spoof form are provably UNCHANGED. Pure string cases -- zero OS
// calls in the function, so valid on any host. Mirrors
// internal/generators/guard_path_normalize_test.go
// (TestNormalizeGuardPath_WindowsSeparators).
test('normalizeGuardPath canonicalizes "\\" only for drive-letter anchored input', () => {
  const cases = [
    // --- drive-letter anchored: canonicalized ---
    ['backslash drive path', 'C:\\Users\\x\\guard.sh', 'C:/Users/x/guard.sh'],
    ['forward-slash drive path unchanged shape', 'C:/Users/x/guard.sh', 'C:/Users/x/guard.sh'],
    ['mixed separators, the exact ML-7A trigger', 'C:\\Users\\RUNNER~1\\AppData\\Local\\Temp\\TestGBGDedup1234567//.trackfw/scripts/trackfw-git-branch-guard.sh', 'C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh'],
    ['computed via join, all backslash', 'C:\\Users\\RUNNER~1\\AppData\\Local\\Temp\\TestGBGDedup1234567\\.trackfw\\scripts\\trackfw-git-branch-guard.sh', 'C:/Users/RUNNER~1/AppData/Local/Temp/TestGBGDedup1234567/.trackfw/scripts/trackfw-git-branch-guard.sh'],
    ["doc-comment's own $HOME-trailing-slash trigger", 'C:\\Users\\foo\\\\.trackfw\\scripts\\trackfw-git-branch-guard.sh', 'C:/Users/foo/.trackfw/scripts/trackfw-git-branch-guard.sh'],
    ['lowercase drive letter', 'd:\\a\\b', 'd:/a/b'],

    // --- valid UNC: byte-for-byte unchanged, including its backslashes ---
    ['valid UNC', '\\\\servidor\\share\\guard.sh', '\\\\servidor\\share\\guard.sh'],
    ['valid UNC, different server', '\\\\other\\share\\guard.sh', '\\\\other\\share\\guard.sh'],

    // --- POSIX-typed UNC-equivalent: pre-existing "//" collapse, untouched by this ML ---
    ['POSIX-typed UNC equivalent collapses like any // input', '//servidor/share/guard.sh', '/servidor/share/guard.sh'],

    // --- degenerate / invalid UNC: not anchored, no translation, no change ---
    ['bare double backslash', '\\\\', '\\\\'],
    ['double backslash no share segment', '\\\\x', '\\\\x'],
    ['server "." is not a hostname', '\\\\.\\x', '\\\\.\\x'],
    ['server ".." is not a hostname', '\\\\..\\evil', '\\\\..\\evil'],
    ['doubled backslash mid-string produces empty share', '\\\\..\\\\evil', '\\\\..\\\\evil'],

    // --- adversarial corpus from the hades-tf ML-3A/3B barrier ---
    ['homoglyph fullwidth C, not ASCII', '\uFF43:\\Users\\x\\guard.sh', '\uFF43:\\Users\\x\\guard.sh'],
    ['zero-width space before drive letter', '\u200bC:\\Users\\x\\guard.sh', '\u200bC:\\Users\\x\\guard.sh'],
    ['digit before colon, not a drive letter', '1:\\Users\\x\\guard.sh', '1:\\Users\\x\\guard.sh'],
    ['leading space before drive letter', ' C:\\Users\\x\\guard.sh', ' C:\\Users\\x\\guard.sh'],
    ['drive-relative, no separator after colon', 'C:foo\\bar', 'C:foo\\bar'],
    ['embedded newline, only backslash bytes change', 'C:\\x\ny\\z', 'C:/x\ny/z'],

    // --- plain POSIX / relative input with a literal backslash byte: must NOT be touched ---
    ['literal backslash in a POSIX segment name is a filename byte, not a separator', '/home/alice/weird\\name/guard.sh', '/home/alice/weird\\name/guard.sh'],
    ['relative path with backslash, no drive letter, untouched', 'scripts\\guard.sh', 'scripts\\guard.sh'],
  ]
  for (const [name, input, want] of cases) {
    assert.equal(normalizeGuardPath(input), want, `${name}: normalizeGuardPath(${JSON.stringify(input)})`)
  }
})

test('hasValidUNCPrefix table', () => {
  const cases = [
    ['\\\\servidor\\share\\guard.sh', true],
    ['\\\\servidor\\share', true],
    ['\\\\', false],
    ['\\\\x', false],
    ['\\\\.\\x', false],
    ['\\\\..\\evil', false],
    ['\\\\..\\\\evil', false],
    ['//servidor/share', false],
    ['C:\\Users\\x', false],
    ['', false],
  ]
  for (const [input, want] of cases) {
    assert.equal(hasValidUNCPrefix(input), want, `hasValidUNCPrefix(${JSON.stringify(input)})`)
  }
})

// Non-loosening control specific to ML-7B: the new equalities introduced by
// canonicalizing "\" must be exactly "same drive-anchored path, different
// separator style" -- nothing else.
test('samePathCommand: Windows separator canonicalization does not over-match', () => {
  assert.equal(samePathCommand('C:\\Users\\x\\guard.sh', 'C:/Users/x/guard.sh'), true)
  const cases = [
    ['\\\\servidor\\share\\guard.sh', '/servidor/share/guard.sh'],
    ['\\\\servidor\\share\\guard.sh', '\\servidor\\share\\guard.sh'],
    ['\\\\servidor\\share\\guard.sh', '//servidor/share/guard.sh'],
    ['\\\\servidor\\share\\guard.sh', '\\\\otherserver\\share\\guard.sh'],
    ['C:\\Users\\alice\\guard.sh', 'C:\\Users\\bob\\guard.sh'],
    ['/home/alice/weird\\name/guard.sh', '/home/alice/weird/name/guard.sh'],
  ]
  for (const [a, b] of cases) {
    assert.equal(samePathCommand(a, b), false, `samePathCommand(${a}, ${b})`)
  }
})
