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

// findMLs locates every `### ML-` heading inside [startLine, endLine) and returns
// { id, lines } for each, where `lines` is the ML body (heading excluded).
function findMLs(lines, startLine, endLine) {
  const headings = []
  for (let i = startLine; i < endLine; i++) {
    if (/^### ML-/.test(lines[i])) {
      const m = /^### (\S+)/.exec(lines[i])
      headings.push({ id: m[1], headingLine: i })
    }
  }
  return headings.map((h, idx) => {
    let end = endLine
    for (let j = h.headingLine + 1; j < endLine; j++) {
      if (/^### /.test(lines[j]) || /^## /.test(lines[j])) { end = j; break }
    }
    return { id: h.id, lines: lines.slice(h.headingLine + 1, end) }
  })
}

// mlCompletionStatus applies rule 3 — completion is a `**Status:**` line whose
// remainder contains ✅. Returns { complete, marker } where marker is "missing"
// when no `**Status:**` line exists at all.
function mlCompletionStatus(mlLines) {
  for (const raw of mlLines) {
    const line = raw.trim()
    const m = /^\*\*Status:\*\*(.*)$/.exec(line)
    if (m) {
      const marker = m[1].trim()
      return { complete: marker.includes('✅'), marker: marker.length > 0 ? marker : 'missing' }
    }
  }
  return { complete: false, marker: 'missing' }
}

// mlAcceptanceEvidence applies rule 4. Returns { hasBlock, total, unmet }.
function mlAcceptanceEvidence(mlLines) {
  let blockStart = -1
  for (let i = 0; i < mlLines.length; i++) {
    if (/^\*\*Crit[ée]rios de aceite:\*\*/.test(mlLines[i].trim())) { blockStart = i; break }
  }
  if (blockStart === -1) return { hasBlock: false, total: 0, unmet: 0 }

  let blockEnd = mlLines.length
  for (let j = blockStart + 1; j < mlLines.length; j++) {
    if (/^\*\*/.test(mlLines[j].trim())) { blockEnd = j; break }
  }

  const blockLines = mlLines.slice(blockStart + 1, blockEnd)
    .map(l => l.trim())
    .filter(l => l.length > 0)

  const criteria = blockLines.filter(l => /^- \[/.test(l))
  if (criteria.length === 0) return { hasBlock: false, total: 0, unmet: 0 }

  const unmet = criteria.filter(l => /^- \[ \]/.test(l))
  return { hasBlock: true, total: criteria.length, unmet: unmet.length }
}

// parseGates applies rule 5. Returns { commands }. Throws UsageError naming the
// offending line number for an unterminated fence.
function parseGates(lines, startLine, endLine) {
  let markerLine = -1
  for (let i = startLine; i < endLine; i++) {
    if (lines[i].trim() === '**Gates da wave:**') { markerLine = i; break }
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
    const { complete, marker } = mlCompletionStatus(ml.lines)
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
    const ev = mlAcceptanceEvidence(ml.lines)
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
    const result = spawnSync(command, {
      shell: true,
      cwd,
      encoding: 'utf8',
      stdio: 'pipe',
    })
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
  const lines = content.split('\n')

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
module.exports.roadmapTrustForGates = roadmapTrustForGates
module.exports.evalMlsComplete = evalMlsComplete
module.exports.evalAcceptanceEvidence = evalAcceptanceEvidence
module.exports.evalGates = evalGates
module.exports.evalValidate = evalValidate
module.exports.buildDoc = buildDoc
module.exports.runBarrier = runBarrier
