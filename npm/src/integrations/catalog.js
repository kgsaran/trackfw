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

module.exports = { catalog, items, target, surfaceFor, readAsset }
