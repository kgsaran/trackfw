'use strict'

const { Command } = require('commander')
const {
  runDoctor,
  UNREGISTERED_WRITE,
  HAND_MODIFIED,
  UNKNOWN_CONTENT,
  REQUIRED_STATUS_CHECKS_MISSING,
  ENFORCE_ADMINS_DISABLED,
  HOOKS_PATH_NEUTRALIZED,
  NOT_EVALUATED,
} = require('../integrations/doctor')
const { runScaffoldDoctor, SCAFFOLD_DIVERGENT, SCAFFOLD_MISSING, SCAFFOLD_WRONG_MODE } = require('../integrations/scaffold_doctor')
const { runDoctorRemote } = require('../integrations/doctor_remote')
const projectConfig = require('../config')

function printReport(findings) {
  if (findings.length === 0) {
    return 'trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact and all scaffold templates.'
  }
  const unregistered = findings.filter(finding => finding.finding === UNREGISTERED_WRITE).length
  const handModified = findings.filter(finding => finding.finding === HAND_MODIFIED).length
  const unknownContent = findings.filter(finding => finding.finding === UNKNOWN_CONTENT).length
  const scaffoldDivergent = findings.filter(finding => finding.finding === SCAFFOLD_DIVERGENT).length
  const scaffoldMissing = findings.filter(finding => finding.finding === SCAFFOLD_MISSING).length
  const scaffoldWrongMode = findings.filter(finding => finding.finding === SCAFFOLD_WRONG_MODE).length
  const requiredChecksMissing = findings.filter(finding => finding.finding === REQUIRED_STATUS_CHECKS_MISSING).length
  const enforceAdminsDisabled = findings.filter(finding => finding.finding === ENFORCE_ADMINS_DISABLED).length
  const hooksPathNeutralized = findings.filter(finding => finding.finding === HOOKS_PATH_NEUTRALIZED).length
  const notEvaluated = findings.filter(finding => finding.finding === NOT_EVALUATED).length
  const lines = [
    `trackfw doctor: ${findings.length} finding(s) -- ${unregistered} unregistered-write, ${handModified} hand-modified, ${unknownContent} unknown-content, ${scaffoldDivergent} scaffold-divergent, ${scaffoldMissing} scaffold-missing, ${scaffoldWrongMode} scaffold-wrong-mode, ${requiredChecksMissing} required-status-checks-missing, ${enforceAdminsDisabled} enforce-admins-disabled, ${hooksPathNeutralized} hooks-path-neutralized, ${notEvaluated} not-evaluated`,
    '',
  ]
  for (const finding of findings) {
    lines.push(`[${finding.finding}] ${finding.destination}`, `  remedy: ${finding.remedy}`, '')
  }
  return lines.join('\n').replace(/\n$/, '')
}

const cmd = new Command('doctor')
cmd.description('Detect artifacts on disk missing from the manifest, distinguishing hand-modified artifacts from unknown content')
cmd.option('--json', 'Emit findings as a JSON array instead of the text report')
cmd.option('--remote', 'Also check GitHub branch protection (required_status_checks, enforce_admins) and local core.hooksPath neutralization. Requires network + a GitHub credential with admin access; absence of either is reported as not-evaluated, never as a pass (ADR-2026-09-02).')
cmd.action(async options => {
  const projectRoot = process.cwd()
  const catalogFindings = runDoctor({ agentModels: projectConfig.load().agentModels || {} })
  // Scaffold coverage (ADR-2026-08-27): compare scaffold artifacts on disk against
  // the templates the current binary would generate, using the project's own
  // trackfw.yaml. No manifest entry is written or read (AC3).
  const scaffoldFindings = runScaffoldDoctor(projectRoot)
  let findings = [...catalogFindings, ...scaffoldFindings]
  // Remote modality (ADR-2026-09-02, ML-3A): opt-in only — never runs without --remote, so
  // doctor's offline default is unchanged.
  if (options.remote) {
    findings = [...findings, ...runDoctorRemote({ configForge: projectConfig.load().forge || '', repoDir: projectRoot })]
  }
  if (options.json) {
    process.stdout.write(`${JSON.stringify(findings, null, 2)}\n`)
    return
  }
  process.stdout.write(`${printReport(findings)}\n`)
})

module.exports = cmd
