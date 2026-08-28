'use strict'

const { Command } = require('commander')
const { runDoctor, UNREGISTERED_WRITE, HAND_MODIFIED, UNKNOWN_CONTENT } = require('../integrations/doctor')
const { runScaffoldDoctor, SCAFFOLD_DIVERGENT, SCAFFOLD_MISSING, SCAFFOLD_WRONG_MODE } = require('../integrations/scaffold_doctor')
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
  const lines = [
    `trackfw doctor: ${findings.length} finding(s) -- ${unregistered} unregistered-write, ${handModified} hand-modified, ${unknownContent} unknown-content, ${scaffoldDivergent} scaffold-divergent, ${scaffoldMissing} scaffold-missing, ${scaffoldWrongMode} scaffold-wrong-mode`,
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
cmd.action(async options => {
  const projectRoot = process.cwd()
  const catalogFindings = runDoctor({ agentModels: projectConfig.load().agentModels || {} })
  // Scaffold coverage (ADR-2026-08-27): compare scaffold artifacts on disk against
  // the templates the current binary would generate, using the project's own
  // trackfw.yaml. No manifest entry is written or read (AC3).
  const scaffoldFindings = runScaffoldDoctor(projectRoot)
  const findings = [...catalogFindings, ...scaffoldFindings]
  if (options.json) {
    process.stdout.write(`${JSON.stringify(findings, null, 2)}\n`)
    return
  }
  process.stdout.write(`${printReport(findings)}\n`)
})

module.exports = cmd
