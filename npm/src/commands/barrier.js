'use strict'

// trackfw barrier <roadmap> --wave <n> [--json]
//
// Deterministic core of the wave-release barrier. Stack-agnostic: every executable
// check comes from the roadmap itself (see docs/cli-parity.md → `## trackfw barrier`).
// This command never assumes a build tool, a test runner or a parity rule, and it
// never performs git operations or invokes specialist agents — that orchestration
// lives in the `/trackfw:barrier` slash command.

const { Command } = require('commander')
const fs = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')
const config = require('../config')
const validator = require('../validator')

// UsageError signals a resolution/parsing failure that must map to exit code 2 —
// distinct from a "blocked" (exit 1) evaluation result.
class UsageError extends Error {}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap resolution
// ────────────────────────────────────────────────────────────────────────────

// resolveRoadmapFile finds <roadmapArg> (basename with or without .md) under
// wip/ then done/ (both flat and by_agent layouts). Throws UsageError naming the
// roadmap exactly as the user typed it (no .md normalization) when not found —
// pinned literally by docs/cli-parity.md (`## trackfw barrier`).
function resolveRoadmapFile(cfg, roadmapArg) {
  const base = roadmapArg.endsWith('.md') ? roadmapArg : `${roadmapArg}.md`
  const dirs = [...validator.resolveWIPDirs(cfg), ...validator.resolveDoneDirs(cfg)]
  for (const dir of dirs) {
    const candidate = path.join(config.expandPath(dir), base)
    if (fs.existsSync(candidate) && fs.statSync(candidate).isFile()) {
      return { path: candidate, basename: base }
    }
  }
  throw new UsageError(`roadmap "${roadmapArg}" not found in wip/ nor done/ under ${cfg.roadmapDir}`)
}

// ────────────────────────────────────────────────────────────────────────────
// Wave label grammar (pinned by docs/cli-parity.md, "Wave label grammar" section)
// ────────────────────────────────────────────────────────────────────────────

// Scan regex (mirrors Go's waveHeadingRe): catches any "## Wave <token> " line.
// The trailing space is part of rule 1 — a heading whose label touches EOL is not
// recognised as a wave heading (neither valid nor malformed).
const WAVE_SCAN_RE = /^## Wave (\S+) /

// Grammar: <integer>[-<suffix>] where integer >= 0 and suffix is [a-z0-9]+ (lowercase).
// Valid: 0, 1, 2, 2-bis, 2-hotfix, 10-a2. 0 is the Wave 0 threat-model convention
// (docs/cli-parity.md § "Wave label grammar").
// Invalid: X, 2-BIS, -bis, 2-, 2-bis-ter.
const WAVE_LABEL_RE = /^(\d+)(?:-([a-z0-9]+))?$/

// isValidWaveLabel returns true iff token matches the grammar above.
// Exported so unit tests can assert the full table of valid/invalid examples.
// Shared by both guard points (the --wave flag validation and the heading
// pre-pass in findWave) — a single function, unlike Go's two duplicated checks.
function isValidWaveLabel(token) {
  const m = WAVE_LABEL_RE.exec(String(token))
  if (!m) return false
  return parseInt(m[1], 10) >= 0
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap parsing rules (string-level, see docs/cli-parity.md)
// ────────────────────────────────────────────────────────────────────────────

// findWave locates the `## Wave <label> ` heading matching waveLabel and returns its
// line range [startLine, endLine) within `lines`. Performs a full pre-pass over all
// wave headings (mirroring Go's parseWaves): throws UsageError for the first malformed
// heading regardless of position — this ensures a typo like `## Wave X — ...` aborts
// the entire document (ADR decision 16, non-vacuity). Throws UsageError naming the
// wave label and the resolved roadmap basename when not found (pinned literally by
// docs/cli-parity.md). roadmapBasename is optional so unit tests that only exercise
// parsing can omit it; the CLI path always supplies it.
function findWave(lines, waveLabel, roadmapBasename) {
  // Pre-pass: validate ALL wave headings and collect them.
  // Abort immediately on the first malformed heading — this is intentional and
  // mirrors Go's parseWaves() full-scan approach (ADR decision 16).
  const waves = []
  for (let i = 0; i < lines.length; i++) {
    const attempt = WAVE_SCAN_RE.exec(lines[i])
    if (!attempt) continue
    const token = attempt[1]
    if (!isValidWaveLabel(token)) {
      throw new UsageError(`malformed wave heading at line ${i + 1}: "${token}" is not a valid wave label`)
    }
    let endLine = lines.length
    for (let j = i + 1; j < lines.length; j++) {
      if (/^## /.test(lines[j])) { endLine = j; break }
    }
    waves.push({ label: token, startLine: i, endLine })
  }

  // Find the requested wave by exact label match (no prefix/fuzzy matching).
  const wave = waves.find(w => w.label === String(waveLabel))
  if (!wave) {
    throw new UsageError(`wave ${waveLabel} not found in roadmap "${roadmapBasename}"`)
  }
  return { startLine: wave.startLine, endLine: wave.endLine }
}

// CRITERIA_HEADER_RE accepts both the canonical English header (ADR
// 2026-08-29 decision 1) and the Portuguese one, which remains accepted with
// no removal date (decision 2 — 99/143 roadmaps in the corpus use it,
// including artifacts in done/). The `^` anchor is load-bearing: an
// unanchored match would casar dentro de prosa ou de cerca de código, turning
// a quoted literal into forged acceptance evidence.
const CRITERIA_HEADER_RE = /^\*\*(?:Acceptance criteria|Crit[ée]rios de aceite):\*\*/

// STATUS_VOCABULARY is the closed set of first-token markers recognised as
// "complete" (ADR decision 3): the checkmark emoji, and the English/Portuguese
// words, matched after case-folding and diacritics-folding. Deliberately
// closed and explicit — "feito", "ok", "finalizado" are out (ADR, Alternatives
// Considered — "accept any non-empty status" is the rejected no-op design).
const STATUS_VOCABULARY = new Set(['✅', 'done', 'concluido'])

// VS16 is the single variation selector this module treats as cosmetic
// noise: the one emoji keyboards insert after "checkmark" to force
// text-style presentation (ADR 2026-08-29 decision 9, exception clause). No
// other variation selector or combining mark gets this treatment.
const VS16 = '\u{FE0F}'

// stripVS16 removes only U+FE0F occurrences from token. Deliberately
// narrower than "strip every variation selector" or "strip every Mn": VS16
// is the one documented cosmetic exception (ADR decision 9); anything else
// of category Mn is now a rejection, not a fold (see
// hasDisallowedCombiningMark).
function stripVS16(token) {
  return token.split(VS16).join('')
}

// hasDisallowedCombiningMark reports whether token (with VS16 already
// removed) contains any Unicode category Mn (Mark, Nonspacing) codepoint.
// ADR 2026-08-29 decision 9: after the single VS16 exception, any combining
// mark on the first status token is rejected outright \u2014 the ML is NOT
// complete \u2014 rather than folded away. A vocabulary this small exists to
// refuse ambiguity; silently folding combining marks reopens exactly the
// ambiguity it exists to close ("d<U+1DC0>one" must not read as "done").
//
// ORDER IS LOAD-BEARING: this check MUST run on the token BEFORE NFD
// decomposition, not after. "Conclu\u00eddo" in its authored (NFC) form has
// no literal Mn codepoint \u2014 the accented "\u00ed" is a single
// precomposed codepoint. NFD-decomposing it *produces* a trailing Mn
// (U+0301, COMBINING ACUTE ACCENT) that normalizeStatusToken then strips
// for vocabulary matching. Running this rejection check on the
// already-decomposed string would treat that legitimate,
// vocabulary-sanctioned accent the same as an injected combining mark and
// reject "Conclu\u00eddo" outright, breaking AC15's own positive case.
// Checking the raw, pre-decomposition token instead lets it through here
// (no literal Mn present) while still catching a combining mark authored
// directly onto the token, which is category Mn in either form, decomposed
// or not.
function hasDisallowedCombiningMark(token) {
  return /\p{Mn}/u.test(token)
}

// normalizeStatusToken folds diacritics and lower-cases the token for
// vocabulary comparison. Uses the \p{Mn} Unicode property escape (General
// Category "Mark, Nonspacing") instead of a hand-enumerated code point range,
// to match Go's runes.In(unicode.Mn) and Python's unicodedata.category(ch) \u2014
// all three fold every combining mark that survives to this point, which,
// after hasDisallowedCombiningMark has already run, only happens as a
// byproduct of NFD-decomposing an accented Latin letter (e.g. "Conclu\u00eddo"
// -> "Concluido"). Callers MUST have already rejected via
// hasDisallowedCombiningMark before calling this \u2014 see that function's
// doc comment for why.
function normalizeStatusToken(token) {
  return token.normalize('NFD').replace(/\p{Mn}/gu, '').toLowerCase()
}

// statusIsComplete implements rule 3 by FIRST TOKEN, not substring (ADR
// decision 3, AC8/AC9/AC14). marker is the already-trimmed remainder of the
// "**Status:**" line. Splitting on /\s+/ treats U+00A0 (NBSP) as a separator \u2014
// matching the accepted "NBSP separator" case \u2014 while a zero-width character
// (U+200B, not matched by \s) stays glued to the token and safely causes
// rejection (a usability false-negative, not a security concern).
//
// This is the fix for vault/notes/adr-status-substring-livre-falso-positivo-2026-08-01.md:
// `marker.includes('checkmark')` would classify "**Status:** [pending] checkmark" as
// complete (reproduced live against 7.3.0 \u2014 ADR decision 8). First-token
// comparison rejects it because the first token is the pending glyph, not the marker.
//
// ADR decision 9: VS16 is stripped first (the one cosmetic exception), then
// any remaining combining mark on the raw token rejects outright \u2014 see
// hasDisallowedCombiningMark for why this must happen before NFD.
function statusIsComplete(marker) {
  const trimmed = marker.trim()
  if (trimmed.length === 0) return false
  const first = stripVS16(trimmed.split(/\s+/)[0])
  if (hasDisallowedCombiningMark(first)) return false
  return STATUS_VOCABULARY.has(normalizeStatusToken(first))
}

// splitRoadmapLines is the single boundary where the raw file content becomes
// the array every marker regex operates on. It normalizes CRLF line endings
// by stripping a trailing "\r" from each line produced by splitting on
// "\n" — every downstream marker (ML heading, "**Status:**", acceptance
// header, criterion lines, "**Gates da wave:**", the fence delimiter) then
// sees the same content it would see for an LF-only file. It does NOT handle
// a lone-CR (old-Mac-style) file: splitting on "\n" alone leaves such a file
// as one giant line, same as internal/commands/barrier.go's splitRoadmapLines.
// Python's universal-newlines read does handle lone CR — a known, accepted
// asymmetry; issue #216 (the defect motivating this fix) is CRLF specifically.
//
// Unlike Go and Python, this runtime DOES depend on this normalization: JS
// regex "." excludes "\r" (it is a LineTerminator per the ECMAScript spec,
// unlike Go's RE2 "."), so `/^\*\*Status:\*\*(.*)$/.exec("**Status:** done\r")`
// returns null — a CRLF roadmap with every ML fully completed was reported as
// "not complete (status: missing)" (mirrors internal/commands/barrier.go
// splitRoadmapLines and the universal-newline read in
// pypi/trackfw/commands/barrier.py).
//
// Only the trailing "\r" immediately before the split point is stripped —
// this must never be confused with per-line indentation trimming, which
// ML-1B deliberately removed: leading whitespace on a marker line still fails
// to match (markers are anchored at column 0, untouched by this function).
function splitRoadmapLines(content) {
  return content.split('\n').map((line) => (line.endsWith('\r') ? line.slice(0, -1) : line))
}

// detectFenceMarker inspects a whitespace-trimmed line and reports whether it
// opens or closes a CommonMark-style fence: a run of 3+ identical backtick
// (`) or tilde (~) characters at the start of the line. Returns
// { char, length } or null. ADR decision (ML-1B, achado 1): CommonMark
// defines a fence as 3+ of the SAME character (backtick or tilde), closed by
// a run of the same character with length >= the opening run — masking only
// "```" left both "~~~" fences and 4+-backtick fences (whose interior can
// nest a 3-backtick block) unmasked, which is the escape route a hostile
// roadmap would use.
function detectFenceMarker(trimmed) {
  if (trimmed.length === 0) return null
  const first = trimmed[0]
  if (first !== '`' && first !== '~') return null
  let i = 0
  while (i < trimmed.length && trimmed[i] === first) i++
  if (i < 3) return null
  return { char: first, length: i }
}

// computeFenceMask returns, for each line index, whether that line lies
// strictly inside a fenced code block (``` ... ``` or ~~~ ... ~~~, per
// CommonMark: 3+ of the same fence character, closed by a run of the same
// character with length >= the opening run's length). A line that is itself
// a fence delimiter is never reported as "inside" — only the lines between an
// opening and a closing delimiter are masked. ADR decision 7 / AC13: ML
// detection, status and acceptance-header parsing must ignore documentation/
// examples inside a cerca — otherwise a roadmap that cites those literals (as
// this very roadmap, its REQ and its ADR do, repeatedly) is read as real ML
// content. parseGates already has its own, independent fence-matching for the
// "```bash ... ```" gates block and is untouched by this mask.
function computeFenceMask(lines) {
  const mask = new Array(lines.length).fill(false)
  let fenced = false
  let fenceChar = null
  let fenceLen = 0
  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim()
    const marker = detectFenceMarker(trimmed)
    if (!fenced) {
      if (marker) {
        fenced = true
        fenceChar = marker.char
        fenceLen = marker.length
        continue
      }
      continue
    }
    // Currently inside a fence: only a marker of the SAME character with
    // length >= the opening run closes it (a nested shorter/different
    // marker stays masked as interior content) — AND, per CommonMark, a
    // closing fence line contains NOTHING besides the fence run and
    // (already-trimmed) surrounding whitespace: marker.length === trimmed.length.
    // Fix for hades-tf security review (2026-08-29, achado #1 /
    // vault/notes/barrier-fence-closing-trailing-content-bypass-2026-08-29.md):
    // a line like "```trailing-junk" found INSIDE an open fence does not
    // close it in real CommonMark — it stays interior content — but the
    // prior check treated any run of length >= fenceLen as a valid close
    // regardless of trailing text. The opening branch above is unchanged:
    // CommonMark allows an info string after the opening run (` ```bash `).
    if (marker && marker.char === fenceChar && marker.length >= fenceLen && marker.length === trimmed.length) {
      fenced = false
      continue
    }
    mask[i] = true
  }
  return mask
}

// findMLs locates every `### ML-` heading inside [startLine, endLine) and returns
// { id, lines, fenced } for each, where `lines` is the ML body (heading excluded)
// and `fenced` is the fence mask aligned to `lines`. fenceMask defaults to "no
// fences" so existing call sites that omit it keep working (ML-1A, AC13).
function findMLs(lines, startLine, endLine, fenceMask = computeFenceMask(lines)) {
  const headings = []
  for (let i = startLine; i < endLine; i++) {
    if (fenceMask[i]) continue
    if (/^### ML-/.test(lines[i])) {
      const m = /^### (\S+)/.exec(lines[i])
      headings.push({ id: m[1], headingLine: i })
    }
  }
  return headings.map((h, idx) => {
    let end = endLine
    for (let j = h.headingLine + 1; j < endLine; j++) {
      if (fenceMask[j]) continue
      if (/^### /.test(lines[j]) || /^## /.test(lines[j])) { end = j; break }
    }
    return {
      id: h.id,
      lines: lines.slice(h.headingLine + 1, end),
      fenced: fenceMask.slice(h.headingLine + 1, end),
    }
  })
}

// mlCompletionStatus applies rule 3 — completion is by FIRST TOKEN (ADR
// decision 3), not substring. Returns { complete, marker } where marker is
// "missing" when no `**Status:**` line exists at all. Lines inside a fenced
// code block are ignored (ADR decision 7, AC13-a) — `fenced` is aligned to
// `mlLines`. Matched against the RAW line (no per-line .trim()) — an indented
// marker does not count (ML-1B, achado 2): Go and Python already require
// column 0 via `^` against the untrimmed line, and Node's prior .trim() made
// it the only runtime that released a wave on indented markers.
//
// Default omitted on purpose (hades-tf security review, 2026-08-29, achado
// #3 / hefesto-tf): a `fenced = []` default left every `fenced[i]` as
// `undefined` when the caller forgot the argument — indistinguishable from
// "nothing is fenced", silently turning OFF the fence-masking protection this
// very function exists to enforce. Production call sites always pass
// `ml.fenced` (never exploitable today), but a completion gate must fail
// CLOSED on a missing argument, not open. The default below marks every line
// as fenced (masked) instead, so an accidentally omitted argument yields
// "missing"/not-complete rather than "everything counts as real content".
function mlCompletionStatus(mlLines, fenced = mlLines.map(() => true)) {
  for (let i = 0; i < mlLines.length; i++) {
    if (fenced[i]) continue
    const m = /^\*\*Status:\*\*(.*)$/.exec(mlLines[i])
    if (m) {
      const marker = m[1].trim()
      return { complete: statusIsComplete(marker), marker: marker.length > 0 ? marker : 'missing' }
    }
  }
  return { complete: false, marker: 'missing' }
}

// mlAcceptanceEvidence applies rule 4. Returns { hasBlock, total, unmet }.
// Lines inside a fenced code block are ignored throughout — for the header
// search, for the "**" block-end boundary, and for counting criterion lines —
// otherwise a cerca citing the acceptance header/`- [x]` as an example would
// forge acceptance evidence (ADR decision 7, AC13-a). All markers are matched
// against the RAW line (no per-line .trim()) — an indented marker does not
// count (ML-1B, achado 2): Go and Python already require column 0 via `^`
// against the untrimmed line, and Node's prior .trim() made it the only
// runtime that released a wave on indented markers.
//
// Default omitted on purpose — same rationale as mlCompletionStatus above
// (hades-tf security review, 2026-08-29, achado #3 / hefesto-tf): fail
// CLOSED (every line treated as fenced/masked, so `hasBlock: false` and no
// forged criteria are counted) if the caller omits `fenced`, instead of
// silently disabling the fence mask.
function mlAcceptanceEvidence(mlLines, fenced = mlLines.map(() => true)) {
  let blockStart = -1
  for (let i = 0; i < mlLines.length; i++) {
    if (fenced[i]) continue
    if (CRITERIA_HEADER_RE.test(mlLines[i])) { blockStart = i; break }
  }
  if (blockStart === -1) return { hasBlock: false, total: 0, unmet: 0 }

  let blockEnd = mlLines.length
  for (let j = blockStart + 1; j < mlLines.length; j++) {
    if (fenced[j]) continue
    if (/^\*\*/.test(mlLines[j])) { blockEnd = j; break }
  }

  const criteria = []
  for (let i = blockStart + 1; i < blockEnd; i++) {
    if (fenced[i]) continue
    const l = mlLines[i]
    if (/^- \[.\]/.test(l)) criteria.push(l)
  }
  if (criteria.length === 0) return { hasBlock: false, total: 0, unmet: 0 }

  const unmet = criteria.filter(l => /^- \[ \]/.test(l))
  return { hasBlock: true, total: criteria.length, unmet: unmet.length }
}

// GATES_HEADER_RE mirrors Go's gatesHeaderRe / Python's _GATES_HEADER_RE: a
// PREFIX match at column 0, not full-line equality — a header followed by
// trailing prose/whitespace on the same line ("**Gates da wave:** (obrigatórios)")
// or a CRLF-terminated line ("**Gates da wave:**\r") must still be recognised,
// exactly as Go/Python already do.
const GATES_HEADER_RE = /^\*\*Gates da wave:\*\*/

// parseGates applies rule 5. Returns { commands }. Throws UsageError naming the
// offending line number for an unterminated fence. The header marker is
// matched against the RAW line (no per-line .trim()) — an indented
// '**Gates da wave:**' does not count (ML-1B, achado 2): Go and Python
// already require column 0 for this marker.
function parseGates(lines, startLine, endLine) {
  let markerLine = -1
  for (let i = startLine; i < endLine; i++) {
    if (GATES_HEADER_RE.test(lines[i])) { markerLine = i; break }
  }
  if (markerLine === -1) return { commands: [] }

  let i = markerLine + 1
  while (i < endLine && lines[i].trim() === '') i++
  if (i >= endLine || lines[i].trim() !== '```bash') {
    throw new UsageError(`malformed gates block at line ${markerLine + 1}: expected a \`\`\`bash fence`)
  }

  const fenceStart = i
  i++
  const commands = []
  let closed = false
  for (; i < endLine; i++) {
    const trimmed = lines[i].trim()
    if (trimmed === '```') { closed = true; break }
    if (trimmed.length === 0 || trimmed.startsWith('#')) continue
    commands.push(trimmed)
  }
  if (!closed) {
    throw new UsageError(`unterminated gates fence starting at line ${fenceStart + 1}`)
  }
  return { commands }
}

// ────────────────────────────────────────────────────────────────────────────
// Checks (fixed evaluation order: mls_complete, acceptance_evidence, gates, validate)
// ────────────────────────────────────────────────────────────────────────────

function evalMlsComplete(mls, waveLabel) {
  const evidence = []
  const failures = []
  for (const ml of mls) {
    const { complete, marker } = mlCompletionStatus(ml.lines, ml.fenced)
    if (complete) evidence.push(`${ml.id}: ✅`)
    else failures.push(`${ml.id}: not complete (status: ${marker})`)
  }
  const status = mls.length > 0 && failures.length === 0 ? 'passed' : 'blocked'
  // Pinned literally by docs/cli-parity.md ("Wave contains zero MLs" case).
  if (mls.length === 0) failures.push(`wave ${waveLabel}: no ML found`)
  return { name: 'mls_complete', status, evidence, failures }
}

function evalAcceptanceEvidence(mls) {
  const evidence = []
  const failures = []
  for (const ml of mls) {
    const ev = mlAcceptanceEvidence(ml.lines, ml.fenced)
    if (!ev.hasBlock) {
      failures.push(`${ml.id}: no acceptance block`)
    } else if (ev.unmet > 0) {
      failures.push(`${ml.id}: ${ev.unmet} unmet acceptance criteria`)
    } else {
      evidence.push(`${ml.id}: ${ev.total} criteria met`)
    }
  }
  const status = failures.length === 0 ? 'passed' : 'blocked'
  return { name: 'acceptance_evidence', status, evidence, failures }
}

// ────────────────────────────────────────────────────────────────────────────
// Roadmap trust check (AC11, AC12 — docs/cli-parity.md § Trust and --trust-local-gates)
// ────────────────────────────────────────────────────────────────────────────

// roadmapTrustForGates determines whether the gates declared in a roadmap can
// be trusted for execution without --trust-local-gates.
//
// Decision (AC4, AC11): the discriminant is git — a roadmap whose content
// differs from origin/main, or that is absent from origin/main, is untrusted.
//
// Returns { trusted: true } or { trusted: false, failureMsg: '...' }.
// Fail-open when not in a git repo, origin/main not resolvable, or any git
// error other than "path absent from origin/main". See docs/cli-parity.md.
function roadmapTrustForGates(roadmapPath) {
  const roadmapDir = path.dirname(roadmapPath)

  // Step 1: check if we are inside a git repository.
  const revParse = spawnSync('git', ['rev-parse', '--git-dir'], {
    cwd: roadmapDir,
    encoding: 'utf8',
    stdio: 'pipe',
  })
  if (revParse.status !== 0) {
    // Not a git repo → fail-open.
    return { trusted: true }
  }

  // Step 2: get the repository toplevel.
  const topLevel = spawnSync('git', ['rev-parse', '--show-toplevel'], {
    cwd: roadmapDir,
    encoding: 'utf8',
    stdio: 'pipe',
  })
  if (topLevel.status !== 0) {
    return { trusted: true }
  }
  const repoRoot = topLevel.stdout.trim()

  // Step 3: compute repo-relative path (always forward slashes for git).
  const absRoadmap = path.resolve(roadmapPath)
  const relPath = path.relative(repoRoot, absRoadmap).split(path.sep).join('/')

  // Step 4: retrieve the file at origin/main.
  const show = spawnSync('git', ['show', `origin/main:${relPath}`], {
    cwd: repoRoot,
    encoding: 'utf8',
    stdio: 'pipe',
  })
  if (show.status !== 0) {
    // If the path specifically does not exist in origin/main → untrusted.
    const stderr = show.stderr || ''
    if (stderr.includes('does not exist in') || stderr.includes('exists on disk, but not in')) {
      return {
        trusted: false,
        failureMsg: 'gates not evaluated: roadmap is not committed in origin/main — pass --trust-local-gates to evaluate local gates',
      }
    }
    // Other failures (no remote, not fetched) → fail-open.
    return { trusted: true }
  }

  // Step 5: compare content byte-for-byte.
  let localContent
  try {
    localContent = fs.readFileSync(roadmapPath, 'utf8')
  } catch (_) {
    return { trusted: true }
  }
  if (show.stdout !== localContent) {
    return {
      trusted: false,
      failureMsg: 'gates not evaluated: roadmap content differs from origin/main — pass --trust-local-gates to evaluate local gates',
    }
  }
  return { trusted: true }
}

// SH_MISSING_MSG is the pinned failure string for a `gates` check that could not
// be evaluated because `sh` is not on $PATH (AC3, AC4). All three runtimes (Go,
// Node, Python) must emit this byte-for-byte — see docs/cli-parity.md
// "Pinned failure strings for not_evaluated".
const SH_MISSING_MSG =
  'gates not evaluated: sh not found in PATH — install a POSIX shell (e.g. Git Bash, WSL) to evaluate gates'

function evalGates(commands, cwd, trustResult = { trusted: true }) {
  // trustResult: { trusted: true } | { trusted: false, failureMsg: string }
  if (!trustResult.trusted) {
    // Roadmap is not trusted: do not execute gates (AC3, AC14).
    // Report as not_evaluated — distinct from passed and blocked (AC6).
    return {
      name: 'gates',
      status: 'not_evaluated',
      commands,
      evidence: [],
      failures: [trustResult.failureMsg],
    }
  }
  const evidence = []
  const failures = []
  for (const command of commands) {
    // `sh` is invoked explicitly, as argv[0], and resolved through $PATH — NOT
    // spawnSync's `shell: true`, which is pinned to a fixed /bin/sh. This is the
    // same $PATH resolution Go has always used via exec.LookPath, and it is
    // required for Windows: the Git Bash `sh.exe` is never at /bin/sh and is
    // only reachable via $PATH.
    const result = spawnSync('sh', ['-c', command], {
      cwd,
      encoding: 'utf8',
      stdio: 'pipe',
    })
    if (result.error) {
      // The process never started at all (e.g. `sh` missing from $PATH) —
      // distinct from the gate command itself failing inside a running `sh`,
      // which surfaces as a normal exit code (e.g. 127 for "tool not found")
      // and never sets result.error. "Could not measure" is not "measured and
      // failed" (AC3, AC4) — stop immediately: gates after this one were never
      // observed, so they must not appear in evidence or failures.
      return {
        name: 'gates',
        status: 'not_evaluated',
        commands,
        evidence: [],
        failures: [SH_MISSING_MSG],
      }
    }
    const code = result.status === null || result.status === undefined ? 1 : result.status
    if (code === 0) evidence.push(`${command}: exit 0`)
    else failures.push(`${command}: exit ${code}`)
  }
  const status = failures.length === 0 ? 'passed' : 'blocked'
  return { name: 'gates', status, commands, evidence, failures }
}

async function evalValidate() {
  const { violations, warnings } = await validator.validate()
  const summary = `${violations.length} violations, ${warnings.length} warnings`
  if (violations.length === 0) {
    return { name: 'validate', status: 'passed', evidence: [summary], failures: [] }
  }
  return { name: 'validate', status: 'blocked', evidence: [], failures: [summary] }
}

// ────────────────────────────────────────────────────────────────────────────
// Result document
// ────────────────────────────────────────────────────────────────────────────

function isoSeconds(date) {
  return date.toISOString().replace(/\.\d{3}Z$/, 'Z')
}

function buildDoc(roadmapBasename, waveNumber, checks, startedAt, finishedAt) {
  const status = checks.every(c => c.status === 'passed') ? 'passed' : 'blocked'
  const failures = []
  for (const check of checks) {
    for (const f of check.failures) failures.push(`${check.name}: ${f}`)
  }
  return {
    roadmap: roadmapBasename,
    wave: waveNumber,
    status,
    started_at: isoSeconds(startedAt),
    finished_at: isoSeconds(finishedAt),
    checks: checks.map(c => {
      const doc = { name: c.name, status: c.status, evidence: c.evidence, failures: c.failures }
      if (c.name === 'gates') {
        return { name: c.name, status: c.status, commands: c.commands, evidence: c.evidence, failures: c.failures }
      }
      return doc
    }),
    failures,
  }
}

function printTextReport(doc) {
  console.log(`Roadmap: ${doc.roadmap}`)
  console.log(`Wave:    ${doc.wave}`)
  console.log(`Status:  ${doc.status}`)
  console.log('')
  for (const check of doc.checks) {
    console.log(`[${check.status}] ${check.name}`)
    for (const e of check.evidence) console.log(`  ✓ ${e}`)
    for (const f of check.failures) console.log(`  ✗ ${f}`)
  }
  if (doc.failures.length > 0) {
    console.log('')
    console.log('Failures:')
    for (const f of doc.failures) console.log(`  • ${f}`)
  }
}

// ────────────────────────────────────────────────────────────────────────────
// Command
// ────────────────────────────────────────────────────────────────────────────

async function runBarrier(roadmapArg, waveOption, jsonOutput, trustLocalGates) {
  if (waveOption === undefined || waveOption === null || String(waveOption).trim() === '') {
    throw new UsageError('--wave is required')
  }
  const waveLabel = String(waveOption).trim()
  if (!isValidWaveLabel(waveLabel)) {
    throw new UsageError(`invalid --wave "${waveOption}" — not a valid wave label`)
  }

  const cfg = config.load()
  const resolved = resolveRoadmapFile(cfg, roadmapArg)
  const content = fs.readFileSync(resolved.path, 'utf8')
  const lines = splitRoadmapLines(content)

  const startedAt = new Date()
  const wave = findWave(lines, waveLabel, resolved.basename)
  const mls = findMLs(lines, wave.startLine, wave.endLine)
  const gates = parseGates(lines, wave.startLine, wave.endLine)

  // Determine trust for gate execution (AC11, AC12).
  const trustResult = trustLocalGates
    ? { trusted: true }
    : roadmapTrustForGates(resolved.path)

  const checks = []
  checks.push(evalMlsComplete(mls, waveLabel))
  checks.push(evalAcceptanceEvidence(mls))
  checks.push(evalGates(gates.commands, process.cwd(), trustResult))
  checks.push(await evalValidate())
  const finishedAt = new Date()

  const doc = buildDoc(resolved.basename, waveLabel, checks, startedAt, finishedAt)

  if (jsonOutput) {
    console.log(JSON.stringify(doc, null, 2))
  } else {
    printTextReport(doc)
  }

  return doc.status === 'passed' ? 0 : 1
}

function createBarrierCommand() {
  const cmd = new Command('barrier')
  cmd
    .description(
      'Deterministic core of the wave-release barrier: evaluates ML completion, ' +
      'acceptance evidence, wave gates and governance validation for a single wave ' +
      'declared in a roadmap.'
    )
    .argument('<roadmap>', 'Roadmap basename, with or without .md')
    .option('--wave <label>', 'Wave label to evaluate (e.g. 1, 2-bis, 2-hotfix)')
    .option('--json', 'Emit the result document as JSON instead of a text report')
    .option('--trust-local-gates', 'Trust the local roadmap content for gate execution without comparing to origin/main (used by the /trackfw:barrier slash command for WIP roadmaps)')
    .action(async (roadmapArg, options) => {
      try {
        const exitCode = await runBarrier(roadmapArg, options.wave, !!options.json, !!options.trustLocalGates)
        process.exit(exitCode)
      } catch (err) {
        if (err instanceof UsageError) {
          process.stderr.write(`trackfw barrier: ${err.message}\n`)
          process.exit(2)
          return
        }
        process.stderr.write(`trackfw barrier: unexpected error: ${err && err.message ? err.message : err}\n`)
        process.exit(2)
      }
    })

  return cmd
}

module.exports = createBarrierCommand()
module.exports.UsageError = UsageError
module.exports.isValidWaveLabel = isValidWaveLabel
module.exports.resolveRoadmapFile = resolveRoadmapFile
module.exports.findWave = findWave
module.exports.findMLs = findMLs
module.exports.mlCompletionStatus = mlCompletionStatus
module.exports.mlAcceptanceEvidence = mlAcceptanceEvidence
module.exports.parseGates = parseGates
module.exports.statusIsComplete = statusIsComplete
module.exports.computeFenceMask = computeFenceMask
module.exports.splitRoadmapLines = splitRoadmapLines
module.exports.CRITERIA_HEADER_RE = CRITERIA_HEADER_RE
module.exports.roadmapTrustForGates = roadmapTrustForGates
module.exports.evalMlsComplete = evalMlsComplete
module.exports.evalAcceptanceEvidence = evalAcceptanceEvidence
module.exports.evalGates = evalGates
module.exports.evalValidate = evalValidate
module.exports.buildDoc = buildDoc
module.exports.runBarrier = runBarrier
