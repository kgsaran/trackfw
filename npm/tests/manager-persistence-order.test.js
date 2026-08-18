'use strict'

// ADR-2026-08-18: install/update persist the manifest before writing artifact
// bytes; uninstall keeps bytes-before-manifest. This file proves both halves
// with executed evidence, mirroring internal/integrations/manager_persistence_order_test.go:
//
//  1. "install interrupted after manifest persist self-heals..." — genuinely
//     interrupts a subprocess at the ADR seam (manager._afterManifestPersist)
//     and proves the resulting disk state is manifest-ahead-of-disk
//     ('not-installed') and that a plain install (no --force) repairs it.
//     This is also the P4 falsification scenario for ML-1A.
//  2. "update write-phase failure rolls back manifest and bytes..." — proves
//     the rollback still restores both manifest and artifact bytes to a
//     non-empty pre-batch baseline when a normal error (not a crash) happens
//     in the write phase, which now runs after the manifest is persisted.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const { spawnSync } = require('node:child_process')

const { buildPlans, IntegrationManager } = require('../src/integrations')

function roots() {
  const base = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-integrations-order-'))
  const projectRoot = path.join(base, 'project')
  const homeRoot = path.join(base, 'home')
  fs.mkdirSync(projectRoot)
  fs.mkdirSync(homeRoot)
  return { base, projectRoot, homeRoot }
}

test('install interrupted after manifest persist self-heals via a later install (no --force)', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })

  const managerModule = path.join(__dirname, '..', 'src', 'integrations')
  const script = `
    const { IntegrationManager } = require(${JSON.stringify(managerModule)})
    const manager = new IntegrationManager(${JSON.stringify(dirs)})
    manager._afterManifestPersist = () => process.exit(7)
    manager.install([${JSON.stringify(plan)}])
    process.exit(0)
  `
  const result = spawnSync(process.execPath, ['-e', script], { encoding: 'utf8' })
  assert.equal(
    result.status,
    7,
    `expected the interrupted subprocess to exit(7) from _afterManifestPersist; got status=${result.status} stderr=${result.stderr}`
  )

  const file = path.join(dirs.projectRoot, plan.destination)
  assert.equal(fs.existsSync(file), false, 'artifact bytes should be absent after the interrupted write')
  const manifestFile = path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')
  const manifest = JSON.parse(fs.readFileSync(manifestFile, 'utf8'))
  assert.ok(manifest.artifacts[file], 'manifest should already declare the artifact (manifest-first ordering)')
  assert.ok(manifest.artifacts[file].sha256, 'manifest entry should already describe the target content')

  const manager = new IntegrationManager(dirs)
  // Detection: 'not-installed' (self-repairable), never 'modified'/unmanaged.
  assert.equal(manager.inspect([plan])[0].state, 'not-installed')
  // Self-repair: a later install with NO --force succeeds and reaches 'current'.
  assert.doesNotThrow(() => manager.install([plan]))
  assert.equal(manager.inspect([plan])[0].state, 'current')
})

test('update write-phase failure rolls back manifest and bytes to a non-empty baseline', () => {
  const dirs = roots()
  const [plan] = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })
  const manager = new IntegrationManager(dirs)
  manager.install([plan])

  const file = path.join(dirs.projectRoot, plan.destination)
  const manifestFile = path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')
  const baselineBytes = fs.readFileSync(file, 'utf8')
  const baselineManifest = fs.readFileSync(manifestFile, 'utf8')

  const updated = { ...plan, content: `${plan.content}\nupdated\n`, catalogVersion: '9.9.9' }

  // Force the write phase (which now runs after the manifest has already
  // been persisted) to fail only for the artifact's bytes, not the manifest —
  // proving the failure happens post-manifest-persist, exactly the case the
  // inversion newly makes possible.
  const realWrite = manager.atomicWrite.bind(manager)
  manager.atomicWrite = (target, content, mode) => {
    if (target === file) throw new Error('injected artifact write failure')
    realWrite(target, content, mode)
  }
  assert.throws(() => manager.update([updated]), /injected artifact write failure/)
  manager.atomicWrite = realWrite

  assert.equal(fs.readFileSync(file, 'utf8'), baselineBytes, 'rollback did not restore artifact bytes')
  assert.equal(fs.readFileSync(manifestFile, 'utf8'), baselineManifest, 'rollback did not restore manifest bytes')
  assert.equal(manager.inspect([plan])[0].state, 'current')
})

// TestUpdateBatchRollbackRestoresAlreadyWrittenArtifactBytes (mirrors the Go
// test of the same intent) closes a gap the single-artifact test above
// cannot: because the injected failure targets the ONLY artifact in that
// batch, its write never lands in the first place, so "bytes equal
// baseline" holds trivially — nothing was ever overwritten. Here a batch of
// two artifacts updates both: the first artifact's write succeeds (its
// bytes really do change to v2 before the batch fails), the second
// artifact's write fails. Rollback must then genuinely revert the first
// artifact's bytes back to v1 — proving the restore actually restores.
test('update batch rollback restores an already-written artifact, not just an untouched one', () => {
  const dirs = roots()
  const [planA] = buildPlans('agents', { targets: ['claude'], items: ['architect'], scope: 'project' })
  const [planB] = buildPlans('agents', { targets: ['claude'], items: ['backend'], scope: 'project' })
  const manager = new IntegrationManager(dirs)
  manager.install([planA, planB])

  const fileA = path.join(dirs.projectRoot, planA.destination)
  const fileB = path.join(dirs.projectRoot, planB.destination)
  const manifestFile = path.join(dirs.projectRoot, '.trackfw/integrations-manifest.json')
  const baselineA = fs.readFileSync(fileA, 'utf8')
  const baselineB = fs.readFileSync(fileB, 'utf8')
  const baselineManifest = fs.readFileSync(manifestFile, 'utf8')

  const updatedA = { ...planA, content: `${planA.content}\nupdated-a\n`, catalogVersion: '9.9.9' }
  const updatedB = { ...planB, content: `${planB.content}\nupdated-b\n`, catalogVersion: '9.9.9' }

  const realWrite = manager.atomicWrite.bind(manager)
  manager.atomicWrite = (target, content, mode) => {
    if (target === fileB) throw new Error('injected artifact B write failure')
    realWrite(target, content, mode)
  }
  assert.throws(() => manager.update([updatedA, updatedB]), /injected artifact B write failure/)
  manager.atomicWrite = realWrite

  assert.equal(fs.readFileSync(fileA, 'utf8'), baselineA, 'rollback did not restore already-written artifact A bytes')
  assert.equal(fs.readFileSync(fileB, 'utf8'), baselineB, 'artifact B bytes changed even though its write failed')
  assert.equal(fs.readFileSync(manifestFile, 'utf8'), baselineManifest, 'rollback did not restore manifest bytes')
  assert.equal(manager.inspect([planA])[0].state, 'current')
  assert.equal(manager.inspect([planB])[0].state, 'current')
})
