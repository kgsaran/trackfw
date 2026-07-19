'use strict'

const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const SCHEMA_VERSION = 1
const sha256 = content => crypto.createHash('sha256').update(content).digest('hex')
const claimKey = claim => [claim.target, claim.surface, claim.scope, claim.kind, claim.item].join('\u0000')
const cleanClaim = claim => ({ target: claim.target, surface: claim.surface, scope: claim.scope, kind: claim.kind, item: claim.item })

class IntegrationManager {
  constructor({ projectRoot = process.cwd(), homeRoot = os.homedir() } = {}) {
    this.roots = { project: path.resolve(projectRoot), global: path.resolve(homeRoot) }
  }

  manifestPath(scope) { return path.join(this.roots[scope], '.trackfw', 'integrations-manifest.json') }

  resolve(scope, destination) {
    const root = this.roots[scope]
    if (!root) throw new Error(`Unsupported scope: ${scope}`)
    if (typeof destination !== 'string' || destination.includes('\u0000') || destination.includes('\\')) throw new Error(`Unsafe destination: ${destination}`)
    let resolved
    if (destination.startsWith('~/')) {
      if (scope !== 'global') throw new Error('Home destination requires global scope')
      resolved = path.resolve(root, destination.slice(2))
    } else if (path.isAbsolute(destination)) {
      resolved = path.normalize(destination)
    } else {
      if (!destination || path.normalize(destination) !== destination || destination === '.' || destination.startsWith(`..${path.sep}`)) throw new Error(`Unsafe destination: ${destination}`)
      resolved = path.resolve(root, destination)
    }
    const rel = path.relative(root, resolved)
    if (!rel || rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) throw new Error(`Destination is outside ${scope} root: ${destination}`)
    this.assertNoSymlinks(root, resolved)
    this.assertNoSymlinks(root, this.manifestPath(scope))
    return resolved
  }

  assertNoSymlinks(root, destination) {
    let current = destination
    while (true) {
      if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink()) throw new Error(`Symlink path is not allowed: ${current}`)
      if (current === root) return
      const parent = path.dirname(current)
      const rel = path.relative(root, current)
      if (parent === current || rel === '..' || rel.startsWith(`..${path.sep}`)) throw new Error(`Path escapes root: ${destination}`)
      current = parent
    }
  }

  loadManifest(scope) {
    const file = this.manifestPath(scope)
    this.assertNoSymlinks(this.roots[scope], file)
    if (!fs.existsSync(file)) return { schema_version: SCHEMA_VERSION, artifacts: {} }
    const parsed = JSON.parse(fs.readFileSync(file, 'utf8'))
    if (parsed.schema_version !== SCHEMA_VERSION || !parsed.artifacts || Array.isArray(parsed.artifacts)) throw new Error(`Unsupported integration manifest: ${file}`)
    return parsed
  }

  atomicWrite(file, content, mode) {
    const root = this.rootFor(file)
    this.assertNoSymlinks(root, file)
    fs.mkdirSync(path.dirname(file), { recursive: true })
    const tmp = path.join(path.dirname(file), `.${path.basename(file)}.${process.pid}.${crypto.randomBytes(6).toString('hex')}.tmp`)
    try {
      fs.writeFileSync(tmp, content, { mode })
      fs.chmodSync(tmp, mode)
      fs.renameSync(tmp, file)
      fs.chmodSync(file, mode)
    } finally {
      if (fs.existsSync(tmp)) fs.unlinkSync(tmp)
    }
  }

  rootFor(file) {
    const found = Object.values(this.roots).find(root => {
      const rel = path.relative(root, file)
      return rel && rel !== '..' && !rel.startsWith(`..${path.sep}`) && !path.isAbsolute(rel)
    })
    if (!found) throw new Error(`Path is outside integration roots: ${file}`)
    return found
  }

  saveManifest(scope, manifest) {
    const artifacts = {}
    for (const destination of Object.keys(manifest.artifacts).sort()) {
      const artifact = manifest.artifacts[destination]
      artifact.claims = artifact.claims.map(cleanClaim).sort((a, b) => claimKey(a).localeCompare(claimKey(b)))
      artifacts[destination] = artifact
    }
    this.atomicWrite(this.manifestPath(scope), `${JSON.stringify({ schema_version: SCHEMA_VERSION, artifacts }, null, 2)}\n`, 0o600)
  }

  inspect(plans) {
    const manifests = new Map()
    return plans.map(plan => {
      const scope = plan.claim.scope
      if (!manifests.has(scope)) manifests.set(scope, this.loadManifest(scope))
      const file = this.resolve(scope, plan.destination)
      const record = manifests.get(scope).artifacts[file]
      const managed = Boolean(record && record.claims.some(claim => claimKey(claim) === claimKey(plan.claim)))
      if (!fs.existsSync(file)) return { ...plan, destination: file, state: 'not-installed', managed }
      const actual = sha256(fs.readFileSync(file))
      const desired = sha256(plan.content)
      let state
      if (record) {
        if (actual !== record.sha256) state = 'modified'
        else if (actual !== desired || record.catalog_version !== plan.catalogVersion) state = 'outdated'
        else state = 'current'
      } else if (actual === desired) state = 'current'
      else if ((plan.legacyHashes || []).includes(actual)) state = 'outdated'
      else state = 'modified'
      return { ...plan, destination: file, state, managed }
    })
  }

  install(plans, { force = false } = {}) { return this.mutate('install', plans, force) }
  update(plans, { force = false } = {}) { return this.mutate('update', plans, force) }
  uninstall(plans, { force = false } = {}) { return this.mutate('uninstall', plans, force) }

  mutate(operation, plans, force) {
    const resolved = plans.map(plan => ({ plan, file: this.resolve(plan.claim.scope, plan.destination) }))
    const manifests = new Map()
    for (const { plan } of resolved) if (!manifests.has(plan.claim.scope)) manifests.set(plan.claim.scope, this.loadManifest(plan.claim.scope))
    const desiredByFile = new Map()
    for (const item of resolved) {
      const desired = sha256(item.plan.content)
      if (operation !== 'uninstall' && desiredByFile.has(item.file) && desiredByFile.get(item.file) !== desired) throw new Error(`Conflicting content planned for: ${item.file}`)
      desiredByFile.set(item.file, desired)
      this.preflight(operation, item, manifests.get(item.plan.claim.scope), force)
    }
    const snapshots = new Map()
    for (const item of resolved) this.snapshot(snapshots, item.file)
    for (const scope of manifests.keys()) this.snapshot(snapshots, this.manifestPath(scope))
    try {
      for (const item of resolved) this.apply(operation, item, manifests.get(item.plan.claim.scope), force)
      for (const [scope, manifest] of [...manifests].sort(([a], [b]) => a.localeCompare(b))) this.saveManifest(scope, manifest)
    } catch (error) {
      this.rollback(snapshots)
      throw error
    }
    return this.inspect(plans)
  }

  preflight(operation, { plan, file }, manifest, force) {
    const status = this.inspectResolved(plan, file, manifest)
    const record = manifest.artifacts[file]
    const owned = Boolean(record && record.claims.some(claim => claimKey(claim) === claimKey(plan.claim)))
    if (operation === 'install') {
      if (status.state === 'modified' && !force) throw new Error(`Artifact is modified; use --force: ${file}`)
      if (status.state === 'outdated' && owned && !force) throw new Error(`Artifact is outdated; use update: ${file}`)
    } else if (operation === 'update') {
      if (!owned && status.state === 'modified') throw new Error(`Unmanaged artifact does not match a trackfw template: ${file}`)
      if (status.state === 'modified' && !force) throw new Error(`Artifact is modified; use --force: ${file}`)
    } else if (operation === 'uninstall' && owned && status.state === 'modified' && !force) {
      throw new Error(`Artifact is modified; use --force: ${file}`)
    }
  }

  inspectResolved(plan, file, manifest) {
    const record = manifest.artifacts[file]
    const managed = Boolean(record && record.claims.some(claim => claimKey(claim) === claimKey(plan.claim)))
    if (!fs.existsSync(file)) return { state: 'not-installed', managed }
    const actual = sha256(fs.readFileSync(file))
    const desired = sha256(plan.content)
    if (record) {
      if (actual !== record.sha256) return { state: 'modified', managed }
      return { state: actual === desired && record.catalog_version === plan.catalogVersion ? 'current' : 'outdated', managed }
    }
    if (actual === desired) return { state: 'current', managed: false }
    if ((plan.legacyHashes || []).includes(actual)) return { state: 'outdated', managed: false }
    return { state: 'modified', managed: false }
  }

  apply(operation, { plan, file }, manifest, force) {
    let record = manifest.artifacts[file]
    const key = claimKey(plan.claim)
    const owned = Boolean(record && record.claims.some(claim => claimKey(claim) === key))
    if (operation === 'uninstall') {
      if (!owned) return
      record.claims = record.claims.filter(claim => claimKey(claim) !== key)
      if (record.claims.length) return
      if (fs.existsSync(file)) fs.unlinkSync(file)
      delete manifest.artifacts[file]
      this.cleanEmpty(path.dirname(file), this.roots[plan.claim.scope])
      return
    }

    const exists = fs.existsSync(file)
    let actual = exists ? sha256(fs.readFileSync(file)) : ''
    const desired = sha256(plan.content)
    const knownLegacy = (plan.legacyHashes || []).includes(actual)
    let writeDesired = !exists
    if (exists && !owned) writeDesired = (operation === 'update' && actual !== desired) || (force && actual !== desired)
    else if (exists && owned) writeDesired = actual !== desired
    if (!record) record = { destination: file, sha256: '', catalog_version: '', claims: [] }
    if (writeDesired) {
      this.atomicWrite(file, plan.content, 0o644)
      actual = desired
    } else if (exists && !owned && actual !== desired && !knownLegacy && !force) {
      throw new Error(`Unmanaged artifact does not match a trackfw template: ${file}`)
    }
    if (!record.claims.some(claim => claimKey(claim) === key)) record.claims.push(cleanClaim(plan.claim))
    record.sha256 = actual
    record.catalog_version = actual === desired ? plan.catalogVersion : 'legacy'
    manifest.artifacts[file] = record
  }

  snapshot(snapshots, file) {
    if (snapshots.has(file)) return
    if (!fs.existsSync(file)) snapshots.set(file, null)
    else snapshots.set(file, { content: fs.readFileSync(file), mode: fs.statSync(file).mode & 0o777 })
  }

  rollback(snapshots) {
    for (const [file, snapshot] of [...snapshots].reverse()) {
      try {
        if (!snapshot) { if (fs.existsSync(file) && !fs.lstatSync(file).isDirectory()) fs.unlinkSync(file) }
        else this.atomicWrite(file, snapshot.content, snapshot.mode)
      } catch { /* preserve original error */ }
    }
  }

  cleanEmpty(directory, root) {
    while (directory !== root) {
      const rel = path.relative(root, directory)
      if (!rel || rel === '..' || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) return
      if (!fs.existsSync(directory) || fs.lstatSync(directory).isSymbolicLink() || fs.readdirSync(directory).length) return
      fs.rmdirSync(directory)
      directory = path.dirname(directory)
    }
  }
}

module.exports = { IntegrationManager, sha256, claimKey, SCHEMA_VERSION }
