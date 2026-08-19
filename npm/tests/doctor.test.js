'use strict'

// Mirrors internal/integrations/doctor_test.go and internal/commands/doctor_test.go.
// classifyDoctor is a pure function; runDoctor exercises the real
// IntegrationManager/buildPlans path so inspectResolved-equivalent logic is
// proven reused, not reimplemented.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { buildPlans, IntegrationManager } = require('../src/integrations')
const { classifyDoctor, runDoctor, UNREGISTERED_WRITE, HAND_MODIFIED, UNKNOWN_CONTENT } = require('../src/integrations/doctor')

function roots() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-doctor-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { projectRoot, homeRoot }
}

test('classifyDoctor: template match, no manifest entry -> unregistered write', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'current', managed: false, registered: false }
  const findings = classifyDoctor([status])
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, UNREGISTERED_WRITE)
  assert.ok(findings[0].remedy.length > 0)
})

test('classifyDoctor: manifest-owned, hash differs -> hand modified', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'modified', managed: true, registered: true }
  const findings = classifyDoctor([status])
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, HAND_MODIFIED)
})

// Was "not our problem" / length 0 before ML-2C — this is exactly the state
// ML-3A's audit found silently falling outside classifyDoctor's cases,
// which is what makes `agents install`'s preflight refuse with "unmanaged
// artifact". See UNKNOWN_CONTENT's doc comment.
test('classifyDoctor: content matches neither template nor manifest entry -> unknown content', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'modified', managed: false, registered: false }
  const findings = classifyDoctor([status])
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, UNKNOWN_CONTENT)
  assert.ok(findings[0].remedy.includes('unmanaged artifact'))
})

test('classifyDoctor: template match, already registered and owned -> nothing to report', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'current', managed: true, registered: true }
  assert.equal(classifyDoctor([status]).length, 0)
})

test('classifyDoctor: registered under a DIFFERENT claim, content current -> must not be reported as unregistered', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'current', managed: false, registered: true }
  assert.equal(classifyDoctor([status]).length, 0)
})

// The unknown-content analogue of the case above: a destination registered
// under a DIFFERENT claim whose content also mismatches must stay silent
// too — it is that other claim's concern, not this one's, regardless of
// state. This is the discriminant Cenário 72 (check-gates-falsify.sh)
// falsifies for UNKNOWN_CONTENT, mirroring Cenário 71 for UNREGISTERED_WRITE.
test('classifyDoctor: registered under a DIFFERENT claim, content modified -> must not be reported as unknown content', () => {
  const status = { claim: { target: 'claude', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' }, destination: '/proj/x.md', state: 'modified', managed: false, registered: true }
  assert.equal(classifyDoctor([status]).length, 0)
})

test('classifyDoctor: sort key is total across a destination shared by two claims', () => {
  const base = { destination: '/proj/shared.md', state: 'current', managed: false, registered: false }
  const a = { ...base, claim: { target: 'zzz', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' } }
  const b = { ...base, claim: { target: 'aaa', surface: 'cli', scope: 'project', kind: 'agents', item: 'backend' } }
  const findings = classifyDoctor([a, b])
  assert.equal(findings.length, 2)
  assert.equal(findings[0].claim.target, 'aaa')
  assert.equal(findings[1].claim.target, 'zzz')
})

test('runDoctor: empty project reports zero findings', () => {
  const { projectRoot, homeRoot } = roots()
  const findings = runDoctor({ identity: { agents: {} }, projectRoot, homeRoot })
  assert.deepEqual(findings, [])
})

test('runDoctor: finds unregistered write after manifest entry removed, distinguishes it from a later hand edit', () => {
  const { projectRoot, homeRoot } = roots()
  const manager = new IntegrationManager({ projectRoot, homeRoot })
  const [plan] = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project', identity: { agents: {} } })
  manager.install([plan])

  const manifestFile = manager.manifestPath('project')
  const manifest = JSON.parse(fs.readFileSync(manifestFile, 'utf8'))
  assert.equal(Object.keys(manifest.artifacts).length, 1)
  manifest.artifacts = {}
  fs.writeFileSync(manifestFile, JSON.stringify(manifest, null, 2))

  let findings = runDoctor({ identity: { agents: {} }, projectRoot, homeRoot })
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, UNREGISTERED_WRITE)

  // Re-register normally, then hand-edit — must classify differently.
  manager.install([plan], { force: true })
  const destination = manager.resolve('project', plan.destination)
  fs.writeFileSync(destination, 'edited by hand')

  findings = runDoctor({ identity: { agents: {} }, projectRoot, homeRoot })
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, HAND_MODIFIED)
})

test('runDoctor: finds unknown content for a destination never installed through trackfw', () => {
  const { projectRoot, homeRoot } = roots()
  const [plan] = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project', identity: { agents: {} } })
  const destination = path.join(projectRoot, plan.destination)
  fs.mkdirSync(path.dirname(destination), { recursive: true })
  // Never installed (zero manifest entry), content matches neither the
  // catalog template nor any LegacyHashes entry.
  fs.writeFileSync(destination, 'nobody installed this through trackfw')

  const findings = runDoctor({ identity: { agents: {} }, projectRoot, homeRoot })
  assert.equal(findings.length, 1)
  assert.equal(findings[0].finding, UNKNOWN_CONTENT)
  assert.ok(findings[0].remedy.includes('unmanaged artifact'))
})
