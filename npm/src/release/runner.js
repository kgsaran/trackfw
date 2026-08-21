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
const { load: loadConfig } = require('../config')
const { resolve: forgeResolve } = require('../forge/resolve')
const { forgeAdapter } = require('../forge/adapter')
const changelog = require('../changelog')

// Named refusal message builders — kept byte-identical (by construction) to Go's
// releaseTag*Fmt constants (internal/commands/release.go) and Python's RELEASE_TAG_* strings,
// so the ML-2B parity gate can compare all 3 CLIs. Every precondition refusal names what to
// fix — release tag prefers refusing over guessing.
function dirtyTreeMsg(statusOut) {
  return `trackfw release tag refuses to run: working tree is not clean.\n${statusOut}\nCommit your changes (trackfw commit) before tagging a release.`
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

// commitDivergesMsg fires when a LOCAL ref (origin/<forge's default branch>'s resolved sha)
// disagrees with what the forge itself reports for that same branch. This ref is writable inside
// the clone (git update-ref) — the forge is the only source that is not — so a disagreement is
// refused, never silently resolved by picking one side. The BRANCH NAME itself comes from the
// forge unconditionally (no local-vs-forge name check — see the call site) since a fresh/shallow
// clone legitimately has no local opinion on it at all. See ADR-2026-08-19-caminho-governado-
// para-push-forcado-e-tag-de-release.md, Emenda 1.
function commitDivergesMsg(base, localSHA, forgeBase, forgeSHA) {
  return `trackfw release tag refuses to run: local origin/${base} (${localSHA}) diverges from the forge's ${forgeBase} tip (${forgeSHA}). A local ref can be stale or forged — investigate before retrying: git fetch origin --prune`
}

// objectAbsentMsg fires when git show <sha>:<path> fails (object absent locally after the
// fetch that Precondition 2 already ran). Names the path and the sha so the user knows
// exactly what is missing. Never falls back to the working tree. See ADR-2026-08-21-
// release-tag-le-versao-e-changelog-do-commit-ancorado.md.
function objectAbsentMsg(filePath, sha, errMessage) {
  return `trackfw release tag refuses to run: could not read ${filePath} at commit ${sha}: ${errMessage}`
}

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

// defaultReadAtCommit reads a file from a specific commit object (git show <sha>:<path>)
// and returns the content verbatim — stdout is NOT trimmed because callers rely on
// byte-exact content (CHANGELOG sections, version strings with newlines). On any failure
// the error surfaces git's real stderr; there is NO fallback to the working tree.
// See ADR-2026-08-21-release-tag-le-versao-e-changelog-do-commit-ancorado.md.
function defaultReadAtCommit(sha, filePath) {
  const result = spawnSync('git', ['--no-replace-objects', 'show', `${sha}:${filePath}`], { encoding: 'utf8' })
  if (result.status !== 0) {
    const msg = (result.stderr || '').trim() || `git show ${sha}:${filePath} exited with ${result.status}`
    return { content: '', error: new Error(msg) }
  }
  // NOT trimmed — content must be preserved byte-for-byte.
  return { content: result.stdout, error: null }
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
  // readAtCommit reads a file from a specific commit object. Default reads via git show so
  // the authority is the sha (content-addressed), not the working tree. Returns
  // { content: string, error: Error|null } — content is NOT trimmed (byte-exact for parsers).
  const readAtCommit = deps.readAtCommit || defaultReadAtCommit
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
  // base is symref-derived — a LOCAL, gravable ref. Used below only as (a) the value the
  // forge's default_branch must agree with, and (b) input to the local-branch-staleness check,
  // unrelated to the forge and unaffected by it.
  const fetchResult = execGit(['fetch', 'origin', '--prune'])
  if (fetchResult.error) {
    writeErr(fetchFailedMsg(fetchResult.error.message))
    return 1
  }

  const base = defaultBaseBranch(execGit)

  // localSHA (origin/<base>'s local tracking ref) is best-effort and non-fatal: a cross-check
  // candidate against the forge, never the source of the commit target. A failure to resolve it
  // must not block reaching the forge resolution below.
  let localSHA = ''
  const objResult = execGit(['rev-parse', `origin/${base}`])
  if (!objResult.error) {
    localSHA = objResult.stdout.trim()

    const localBranchExists = execGit(['rev-parse', '-q', '--verify', `refs/heads/${base}`])
    if (!localBranchExists.error) {
      const localResult = execGit(['rev-parse', `refs/heads/${base}`])
      if (!localResult.error) {
        const localBranchSHA = localResult.stdout.trim()
        if (localBranchSHA !== localSHA) {
          writeErr(localBranchStaleMsg(base, localBranchSHA, localSHA))
          return 1
        }
      }
    }
  }

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
    // No forge to ask — localSHA is shown purely as an informational hint for the manual
    // fallback text below; the command never publishes on this path.
    writeErr(unsupportedForgeMsg(resolution.forge, tagName, localSHA))
    return 1
  }

  const adapter = forgeAdapter(resolution.forge, availFn)
  if (!adapter.available) {
    // Same reasoning as above: no forge CLI to ask, localSHA is informational only.
    writeErr(noForgeCLIMsg(tagName, localSHA))
    return 1
  }

  // ─── The commit-target comes from the forge, never from a local ref ─────
  // The forge's default_branch is authoritative for the BRANCH NAME — unconditionally, with no
  // refusal if it disagrees with the local symref-derived base (a fresh/shallow clone may have
  // no origin/HEAD symref at all, defaultBaseBranch then falls back to "main"; refusing on that
  // mismatch would be a false refusal against a legitimate repo, not a security check). Only the
  // forge's SHA is cross-checked against a local ref — resolved fresh, keyed to the forge's own
  // branch name, never to the (possibly-forged) local base. See ADR-2026-08-19-caminho-
  // governado-para-push-forcado-e-tag-de-release.md, Emenda 1.
  const repoInfoResp = execForgeAPI('gh', ['api', 'repos/{owner}/{repo}'], '')
  if (repoInfoResp.error) {
    writeErr(`trackfw release tag: gh api failed resolving the repository's default branch from the forge: ${repoInfoResp.error.message}`)
    return 1
  }
  let repoInfo
  try {
    repoInfo = JSON.parse(repoInfoResp.stdout)
  } catch (_) {
    repoInfo = {}
  }
  if (!repoInfo.default_branch) {
    writeErr(`trackfw release tag: could not parse default_branch from the forge's repository response: ${repoInfoResp.stdout}`)
    return 1
  }

  // forgeLocalSHA is resolved fresh against the forge's own branch name — deliberately NOT
  // reusing localSHA above, which was keyed to the symref-derived base and may name a different
  // branch (stale symref, or a fresh clone with no symref at all). Best-effort/non-fatal.
  let forgeLocalSHA = ''
  const forgeObjResult = execGit(['rev-parse', `origin/${repoInfo.default_branch}`])
  if (!forgeObjResult.error) {
    forgeLocalSHA = forgeObjResult.stdout.trim()
  }

  const commitResp = execForgeAPI('gh', ['api', `repos/{owner}/{repo}/commits/${repoInfo.default_branch}`], '')
  if (commitResp.error) {
    writeErr(`trackfw release tag: gh api failed resolving the forge's tip commit for ${repoInfo.default_branch}: ${commitResp.error.message}`)
    return 1
  }
  let commitObj
  try {
    commitObj = JSON.parse(commitResp.stdout)
  } catch (_) {
    commitObj = {}
  }
  if (!commitObj.sha) {
    writeErr(`trackfw release tag: could not parse the forge's commit response for ${repoInfo.default_branch}: ${commitResp.stdout}`)
    return 1
  }

  if (forgeLocalSHA && forgeLocalSHA !== commitObj.sha) {
    writeErr(commitDivergesMsg(repoInfo.default_branch, forgeLocalSHA, repoInfo.default_branch, commitObj.sha))
    return 1
  }

  // objectSHA is now authoritative — resolved from the forge, cross-checked (not sourced)
  // against the local ref above.
  const objectSHA = commitObj.sha

  // ─── Precondition 3: version files in the commit-target must all match ──
  // Content is read from objectSHA via git show, NOT from the working tree. Objects are
  // content-addressed: given a sha from the forge, the content is cyptographically determined —
  // a local edit that was not committed cannot influence the tag message.
  // Absent object → refuse naming sha+path; never fall back to local. See ADR-2026-08-21.
  for (const vf of RELEASE_VERSION_FILES) {
    const readResult = readAtCommit(objectSHA, vf.path)
    if (readResult.error) {
      writeErr(objectAbsentMsg(vf.path, objectSHA, readResult.error.message))
      return 1
    }
    let got
    try {
      got = vf.extract(readResult.content)
    } catch (e) {
      writeErr(`trackfw release tag refuses to run: ${e.message}`)
      return 1
    }
    if (got !== version) {
      writeErr(versionMismatchMsg(vf.label, got, version))
      return 1
    }
  }

  // ─── Precondition 4: CHANGELOG.md in the commit-target has the version's section ──
  // Same anchoring as P3: content comes from objectSHA, never from the working tree.
  const changelogReadResult = readAtCommit(objectSHA, 'CHANGELOG.md')
  if (changelogReadResult.error) {
    writeErr(objectAbsentMsg('CHANGELOG.md', objectSHA, changelogReadResult.error.message))
    return 1
  }
  const sections = changelog.parseSections(changelogReadResult.content)
  let section
  try {
    section = changelog.findVersion(sections, version)
  } catch (e) {
    writeErr(changelogMissingMsg(e.message, version))
    return 1
  }
  const tagMessage = changelog.formatSection(section)

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

// GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX — see ship/runner.js's identical constant/rationale.
const GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX = 'refs/remotes/origin/'

/**
 * defaultBaseBranch resolves the repository's default branch, mirroring Go's
 * defaultBaseBranch (ship.go) exactly: tries symbolic-ref refs/remotes/origin/HEAD, falls back
 * to "main". This is a LOCAL, gravable ref — release tag treats its result as a cross-check
 * candidate only, never as the source of truth for the tag's commit target.
 * @param {function} execGit
 * @returns {string}
 */
function defaultBaseBranch(execGit) {
  const result = execGit(['symbolic-ref', 'refs/remotes/origin/HEAD'])
  if (result.error) return 'main'
  const out = result.stdout.trim()
  if (!out.startsWith(GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX)) return 'main'
  const name = out.slice(GIT_SYMBOLIC_REF_ORIGIN_HEAD_PREFIX.length)
  return name === '' ? 'main' : name
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
