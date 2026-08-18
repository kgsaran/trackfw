'use strict'

const { Command } = require('commander')
const { runDoctor, UNREGISTERED_WRITE, HAND_MODIFIED } = require('../integrations/doctor')

function printReport(findings) {
  if (findings.length === 0) {
    return 'trackfw doctor: no mismatches found -- disk matches the manifest for every catalog-managed artifact.'
  }
  const unregistered = findings.filter(finding => finding.finding === UNREGISTERED_WRITE).length
  const handModified = findings.filter(finding => finding.finding === HAND_MODIFIED).length
  const lines = [`trackfw doctor: ${findings.length} finding(s) -- ${unregistered} unregistered-write, ${handModified} hand-modified`, '']
  for (const finding of findings) {
    lines.push(`[${finding.finding}] ${finding.destination}`, `  remedy: ${finding.remedy}`, '')
  }
  return lines.join('\n').replace(/\n$/, '')
}

const cmd = new Command('doctor')
cmd.description('Detect artifacts on disk missing from the manifest, and distinguish them from hand-modified artifacts')
cmd.option('--json', 'Emit findings as a JSON array instead of the text report')
cmd.action(async options => {
  const findings = runDoctor({})
  if (options.json) {
    process.stdout.write(`${JSON.stringify(findings, null, 2)}\n`)
    return
  }
  process.stdout.write(`${printReport(findings)}\n`)
})

module.exports = cmd
