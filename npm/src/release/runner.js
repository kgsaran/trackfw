'use strict'

/**
 * release/runner.js — Core implementation of `trackfw release tag`.
 *
 * Port of internal/commands/release.go — keep in sync. See
 * ADR-2026-08-19-caminho-governado-para-push-forcado-e-tag-de-release.md for why this exists
 * as a separate command from `ship`: tag is not a branch operation, and ship's governance gate
 * ("REQ + roadmap in wip/") does not apply to release.
 *
 * All git/gh operations are injectable for testability. Publishes via two `gh api` calls
 * (POST git/tags then POST git/refs) — the reference sequence validated in production for
 * v7.1.0 — which preserves the tag's annotation; a plain `git push origin <tag>` from a
 * lightweight local tag would lose it, and the git-branch-guard blocks that push form anyway.
 */

const { spawnSync } = require('child_process')
const fs = require('fs')
const path = require('path')
const { load: loadConfig } = require('../config')
const { resolve: forgeResolve } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')
const changelog = require('../changelog')

// Named refusal message builders — kept byte-identical (by construction) to Go's
// releaseTag*Fmt constants (internal/commands/release.go) and Python's RELEASE_TAG_* strings,
// so the ML-2B parity gate can compare all 3 CLIs. Every precondition refusal names what to
// fix — release tag prefers refusing over guessing.
function dirtyTreeMsg(statusOut) {
  return `trackfw release tag refuses to run: working tree is not clean.\n${statusOut}\nCommit or stash your changes before tagging a release.`
}

function fetchFailedMsg(errMessage) {
  return `trackfw release tag refuses to run: could not fetch origin (${errMessage}). Check your network/credentials and retry.`
}

function localBranchStaleMsg(base, localSHA, remoteSHA) {
  return `trackfw release tag refuses to run: local "${base}" is not up to date with origin/${base} (local ${localSHA}, remote ${remoteSHA}). Run: git pull`
}

function versionMismatchMsg(label, got, want) {
  return `trackfw release tag refuses to run: ${label} has version "${got}", expected "${want}". Update it to match before tagging.`
}

function changelogMissingMsg(underlyingMessage, version) {
  return `trackfw release tag refuses to run: ${underlyingMessage}. Add a "## [${version}] - YYYY-MM-DD" section to CHANGELOG.md before tagging.`
}

function existsLocalMsg(tagName) {
  return `trackfw release tag refuses to run: tag "${tagName}" already exists locally. Delete it first (git tag -d ${tagName}) or choose a different version.`
}

function existsRemoteMsg(tagName) {
  return `trackfw release tag refuses to run: tag "${tagName}" already exists on origin. Choose a different version.`
}

function noForgeCLIMsg(tagName, objectSHA) {
  return `trackfw release tag requires the GitHub CLI (gh) to publish the tag. No forge CLI is available for this repository — install and authenticate gh, or push the tag manually: git tag -a ${tagName} -m "<CHANGELOG.md section>" ${objectSHA} && git push origin ${tagName}`
}

function unsupportedForgeMsg(resolvedForge, tagName, objectSHA) {
  return `trackfw release tag currently only supports GitHub (resolved forge: "${resolvedForge}"). Publishing tag ${tagName} on this forge is not implemented yet — commit to tag: ${objectSHA}. Create ${tagName} through your forge's web UI, or open an issue requesting support for this forge.`
}

const NO_GIT_IDENTITY_MSG =
  'trackfw release tag refuses to run: git config user.name and user.email must be set to create an annotated tag (git config user.name "Your Name" && git config user.email you@example.com).'

// ─── Version file extraction ───────────────────────────────────────────────

const GO_VERSION_RE = /Version\s*=\s*"([^"]+)"/
const PYPROJECT_VERSION_RE = /^version\s*=\s*"([^"]+)"/m
// Matches the try-block fallback in `__version__ = version("trackfw") or "7.1.0"`.
const INIT_TRY_VERSION_RE = /or\s+"([^"]+)"/
// Matches the except-block's `__version__ = "7.1.0"` — distinct from the try-block line, which
// never starts with `__version__ = "` directly (it starts with `__version__ = version(...)`).
const INIT_EXCEPT_VERSION_RE = /__version__\s*=\s*"([^"]+)"/

function extractGoVersion(content) {
  const m = GO_VERSION_RE.exec(content)
  if (!m) throw new Error('could not find Version = "..." in internal/version/version.go')
  return m[1]
}

function extractNpmVersion(content) {
  let pkg
  try {
    pkg = JSON.parse(content)
  } catch (e) {
    throw new Error(`could not parse npm/package.json: ${e.message}`)
  }
  if (!pkg.version) throw new Error('npm/package.json has no "version" field')
  return pkg.version
}

function extractPyprojectVersion(content) {
  const m = PYPROJECT_VERSION_RE.exec(content)
  if (!m) throw new Error('could not find version = "..." in pypi/pyproject.toml')
  return m[1]
}

function extractInitTryVersion(content) {
  const m = INIT_TRY_VERSION_RE.exec(content)
  if (!m) throw new Error('could not find the importlib.metadata fallback version in pypi/trackfw/__init__.py')
  return m[1]
}

function extractInitExceptVersion(content) {
  const m = INIT_EXCEPT_VERSION_RE.exec(content)
  if (!m) throw new Error('could not find the except fallback version in pypi/trackfw/__init__.py')
  return m[1]
}

const RELEASE_VERSION_FILES = [
  { label: 'internal/version/version.go', path: 'internal/version/version.go', extract: extractGoVersion },
  { label: 'npm/package.json', path: 'npm/package.json', extract: extractNpmVersion },
  { label: 'pypi/pyproject.toml', path: 'pypi/pyproject.toml', extract: extractPyprojectVersion },
  { label: 'pypi/trackfw/__init__.py (importlib.metadata fallback)', path: 'pypi/trackfw/__init__.py', extract: extractInitTryVersion },
  { label: 'pypi/trackfw/__init__.py (except fallback)', path: 'pypi/trackfw/__init__.py', extract: extractInitExceptVersion },
]

/** normalizeReleaseVersion strips an optional leading "v"/"V". */
function normalizeReleaseVersion(v) {
  if (v && (v[0] === 'v' || v[0] === 'V')) return v.slice(1)
  return v
}

// ─── Default dependency implementations ────────────────────────────────────

function defaultExecGit(args) {
  const result = spawnSync('git', args, { encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `git ${args.join(' ')} exited with ${result.status}`
    return { stdout: '', error: new Error(msg) }
  }
  return { stdout: (result.stdout || '').trim(), error: null }
}

function defaultReadFile(filePath) {
  return fs.readFileSync(filePath, 'utf8')
}

/**
 * defaultExecForgeAPI runs a forge CLI command feeding stdin and capturing stdout, so the JSON
 * response can be parsed. On failure, surfaces the CLI's real stderr text.
 * @returns {{ stdout: string, error: Error|null }}
 */
function defaultExecForgeAPI(name, args, stdin) {
  const result = spawnSync(name, args, { input: stdin, encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `${name} ${args.join(' ')} exited with ${result.status}`
    return { stdout: (result.stdout || '').trim(), error: new Error(msg) }
  }
  return { stdout: (result.stdout || '').trim(), error: null }
}

/**
 * runReleaseTag implements `trackfw release tag <version>`. Every precondition below is
 * checked before any write — the risk this command carries is publishing a wrong tag to a
 * public repository, so it always refuses rather than guesses.
 * @param {string} versionArg
 * @param {object} deps
 * @returns {number} exit code (0 = success)
 */
function runReleaseTag(versionArg, deps = {}) {
  const execGit = deps.execGit || defaultExecGit
  const readFile = deps.readFile || ((p) => defaultReadFile(path.join(deps.repoDir || '.', p)))
  const writeln = deps.writeln || ((s) => process.stdout.write(s + '\n'))
  const writeErr = deps.writeErr || ((s) => process.stderr.write(`Error: ${s}\n`))
  const configForge = deps.configForge || ''
  const repoDir = deps.repoDir !== undefined ? deps.repoDir : '.'
  const availFn = deps.availFn || undefined
  const execForgeAPI = deps.execForgeAPI || defaultExecForgeAPI

  const version = normalizeReleaseVersion(String(versionArg).trim())
  const tagName = `v${version}`

  // ─── Precondition 1: clean working tree ──────────────────────────────────
  const statusResult = execGit(['status', '--porcelain'])
  if (statusResult.error) {
    writeErr(`could not determine working tree status: ${statusResult.error.message}`)
    return 1
  }
  if (statusResult.stdout.trim() !== '') {
    writeErr(dirtyTreeMsg(statusResult.stdout))
    return 1
  }

  // ─── Precondition 2: default branch up to date with origin ──────────────
  const fetchResult = execGit(['fetch', 'origin', '--prune'])
  if (fetchResult.error) {
    writeErr(fetchFailedMsg(fetchResult.error.message))
    return 1
  }

  const base = defaultBaseBranch(execGit)

  const objResult = execGit(['rev-parse', `origin/${base}`])
  if (objResult.error) {
    writeErr(`could not resolve origin/${base}: ${objResult.error.message}`)
    return 1
  }
  const objectSHA = objResult.stdout.trim()

  const localBranchExists = execGit(['rev-parse', '-q', '--verify', `refs/heads/${base}`])
  if (!localBranchExists.error) {
    const localResult = execGit(['rev-parse', `refs/heads/${base}`])
    if (!localResult.error) {
      const localSHA = localResult.stdout.trim()
      if (localSHA !== objectSHA) {
        writeErr(localBranchStaleMsg(base, localSHA, objectSHA))
        return 1
      }
    }
  }

  // ─── Precondition 3: the 4 version files must all match ─────────────────
  for (const vf of RELEASE_VERSION_FILES) {
    let content
    try {
      content = readFile(vf.path)
    } catch (e) {
      writeErr(`trackfw release tag refuses to run: could not read ${vf.path}: ${e.message}`)
      return 1
    }
    let got
    try {
      got = vf.extract(content)
    } catch (e) {
      writeErr(`trackfw release tag refuses to run: ${e.message}`)
      return 1
    }
    if (got !== version) {
      writeErr(versionMismatchMsg(vf.label, got, version))
      return 1
    }
  }

  // ─── Precondition 4: CHANGELOG.md has the version's section ─────────────
  let changelogContent
  try {
    changelogContent = readFile('CHANGELOG.md')
  } catch (e) {
    writeErr(`trackfw release tag refuses to run: could not read CHANGELOG.md: ${e.message}`)
    return 1
  }
  const sections = changelog.parseSections(changelogContent)
  let section
  try {
    section = changelog.findVersion(sections, version)
  } catch (e) {
    writeErr(changelogMissingMsg(e.message, version))
    return 1
  }
  const tagMessage = changelog.formatSection(section)

  // ─── Precondition 5: tag must not already exist, local or remote ────────
  const localTagExists = execGit(['rev-parse', '-q', '--verify', `refs/tags/${tagName}`])
  if (!localTagExists.error) {
    writeErr(existsLocalMsg(tagName))
    return 1
  }
  const remoteTagResult = execGit(['ls-remote', '--tags', 'origin', `refs/tags/${tagName}`])
  if (remoteTagResult.stdout.trim() !== '') {
    writeErr(existsRemoteMsg(tagName))
    return 1
  }

  // ─── Precondition 6: forge CLI available — GitHub only, for now ─────────
  const remoteURLResult = execGit(['remote', 'get-url', 'origin'])
  const remoteURL = (remoteURLResult.stdout || '').trim()

  let resolution
  try {
    resolution = forgeResolve({ configForge, remoteURL, repoDir })
  } catch (e) {
    writeErr(e.message)
    return 1
  }

  if (resolution.forge !== 'github') {
    writeErr(unsupportedForgeMsg(resolution.forge, tagName, objectSHA))
    return 1
  }

  const adapter = forgeAdapter(resolution.forge, availFn)
  if (!adapter.available) {
    writeErr(noForgeCLIMsg(tagName, objectSHA))
    return 1
  }

  // ─── Tagger identity ──────────────────────────────────────────────────
  const nameResult = execGit(['config', 'user.name'])
  const emailResult = execGit(['config', 'user.email'])
  const name = (nameResult.stdout || '').trim()
  const email = (emailResult.stdout || '').trim()
  if (!name || !email) {
    writeErr(NO_GIT_IDENTITY_MSG)
    return 1
  }

  // ─── Publish: two gh api calls, preserving the annotation ───────────────
  const tagPayload = JSON.stringify({
    tag: tagName,
    message: tagMessage,
    object: objectSHA,
    type: 'commit',
    // RFC3339, no fractional seconds — matches Go's time.Now().UTC().Format(time.RFC3339) and
    // Python's strftime("%Y-%m-%dT%H:%M:%SZ") exactly (Date.prototype.toISOString() emits
    // milliseconds, which those two do not; a byte-for-byte-format gate would catch the drift
    // even though the *value* always differs by wall-clock time anyway).
    tagger: { name, email, date: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z') },
  })

  const tagResp = execForgeAPI('gh', ['api', 'repos/{owner}/{repo}/git/tags', '--method', 'POST', '--input', '-'], tagPayload)
  if (tagResp.error) {
    writeErr(`trackfw release tag: gh api failed creating the tag object: ${tagResp.error.message}`)
    return 1
  }

  let tagObj
  try {
    tagObj = JSON.parse(tagResp.stdout)
  } catch (_) {
    tagObj = {}
  }
  if (!tagObj.sha) {
    writeErr(`trackfw release tag: could not parse the tag object response from gh api: ${tagResp.stdout}`)
    return 1
  }

  const refPayload = JSON.stringify({ ref: `refs/tags/${tagName}`, sha: tagObj.sha })
  const refResp = execForgeAPI('gh', ['api', 'repos/{owner}/{repo}/git/refs', '--method', 'POST', '--input', '-'], refPayload)
  if (refResp.error) {
    writeErr(`trackfw release tag: gh api failed creating the tag ref: ${refResp.error.message}`)
    return 1
  }

  writeln(`Tag published: ${tagName}`)
  writeln(`  tag object: ${tagObj.sha}`)
  writeln(`  commit:     ${objectSHA}`)
  writeln('')
  writeln('release tag complete.')
  return 0
}

/**
 * defaultBaseBranch resolves the repository's default branch, mirroring Go's
 * defaultBaseBranch (ship.go) exactly: tries symbolic-ref refs/remotes/origin/HEAD, falls back
 * to "main".
 * @param {function} execGit
 * @returns {string}
 */
function defaultBaseBranch(execGit) {
  const result = execGit(['symbolic-ref', 'refs/remotes/origin/HEAD'])
  if (result.error) return 'main'
  const out = result.stdout.trim()
  const idx = out.lastIndexOf('/')
  if (idx < 0 || idx + 1 >= out.length) return 'main'
  return out.slice(idx + 1)
}

module.exports = {
  runReleaseTag,
  normalizeReleaseVersion,
  RELEASE_VERSION_FILES,
  extractGoVersion,
  extractNpmVersion,
  extractPyprojectVersion,
  extractInitTryVersion,
  extractInitExceptVersion,
  defaultBaseBranch,
}
