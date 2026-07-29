'use strict'

const fs = require('node:fs')
const path = require('node:path')

const ASSET_ROOT = path.join(__dirname, 'assets')
const catalog = Object.freeze(JSON.parse(fs.readFileSync(path.join(ASSET_ROOT, 'catalog.json'), 'utf8')))

function items(kind) {
  if (kind !== 'agents' && kind !== 'skills') throw new Error(`Unsupported integration kind: ${kind}`)
  return catalog[kind]
}

function target(id) {
  const found = catalog.targets.find(entry => entry.id === id)
  if (!found) throw new Error(`Unsupported target: ${id}`)
  return found
}

function surfaceFor(targetEntry, requested, kind = 'agents') {
  const surfaces = targetEntry.surfaces || []
  const found = requested
    ? surfaces.find(entry => entry.id === requested)
    : surfaces.find(entry => !['legacy', 'unsupported'].includes(entry.capabilities[kind].support_level)) || surfaces[0]
  if (!found) throw new Error(`Unsupported surface ${requested} for target ${targetEntry.id}`)
  return found
}

function readAsset(item) {
  const relative = item.asset.replace(/^assets\//, '')
  return fs.readFileSync(path.join(ASSET_ROOT, relative), 'utf8')
}

// truncateBeforeIdSegment drops the "{{id}}"-bearing path segment and
// everything after it, returning the shared parent directory — mirrors
// internal/integrations/plan.go:truncateBeforeIDSegment.
function truncateBeforeIdSegment(p) {
  const segments = p.split('/')
  const index = segments.findIndex(segment => segment.includes('{{id}}'))
  if (index === -1) return p
  return segments.slice(0, index).join('/')
}

// globalGroupPath returns the tilde-abbreviated directory shared by every
// catalog item of (toolId, kind) at global scope, derived from the catalog's
// own path template rather than any individual installed plan's destination
// — so the reported path never depends on catalog item iteration order. See
// docs/cli-parity.md, "Declared harness targets — pinned list", and the Go
// counterpart internal/integrations/plan.go:GlobalGroupPath.
function globalGroupPath(toolId, kind) {
  const targetEntry = target(toolId)
  const surface = surfaceFor(targetEntry, undefined, kind)
  const installPath = surface.paths[kind].find(entry => entry.scope === 'global')
  if (!installPath) throw new Error(`target ${toolId} has no global ${kind} path`)
  return truncateBeforeIdSegment(installPath.path)
}

module.exports = { catalog, items, target, surfaceFor, readAsset, globalGroupPath }
