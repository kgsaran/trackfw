'use strict'

const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const MANIFEST_VERSION = 1

function sha256(content) {
  return crypto.createHash('sha256').update(content).digest('hex')
}

function claimKey(claim) {
  return [claim.target, claim.surface, claim.scope, claim.kind, claim.item].join('\u0000')
}

class IntegrationManager {
  constructor({ projectRoot = process.cwd(), homeRoot = os.homedir() } = {}) {
    this.roots = { project: path.resolve(projectRoot), global: path.resolve(homeRoot) }
  }

  manifestPath(scope) {
    return path.join(this.roots[scope], '.trackfw', 'integrations-manifest.json')
  }

  resolve(scope, destination) {
    if (!this.roots[scope]) throw new Error(`Unsupported scope: ${scope}`)
    if (typeof destination !== 'string' || destination.includes('\u0000') || destination.includes('\\')) throw new Error(`Unsafe destination: ${destination}`)
    let relative = destination
    if (scope === 'global') {
      if (!relative.startsWith('~/')) throw new Error(`Global destination must start with ~/: ${destination}`)
      relative = relative.slice(2)
    } else if (relative.startsWith('~/') || path.isAbsolute(relative)) {
      throw new Error(`Project destination must be relative: ${destination}`)
    }
    const segments = relative.split('/')
    if (!relative || segments.some(segment => !segment || segment === '.' || segment === '..')) throw new Error(`Unsafe destination: ${destination}`)
    const resolved = path.resolve(this.roots[scope], ...segments)
    const rel = path.relative(this.roots[scope], resolved)
    if (!rel || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) throw new Error(`Destination escapes ${scope} root: ${destination}`)
    this.assertNoSymlinks(this.roots[scope], resolved)
    return resolved
  }

  assertNoSymlinks(root, destination) {
    let current = root
    if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink()) throw new Error(`Symlink root is not allowed: ${root}`)
    const rel = path.relative(root, destination)
    for (const segment of rel.split(path.sep)) {
      current = path.join(current, segment)
      if (fs.existsSync(current) && fs.lstatSync(current).isSymbolicLink()) throw new Error(`Symlink destination is not allowed: ${current}`)
    }
  }

  loadManifest(scope) {
    const file = this.manifestPath(scope)
    this.assertNoSymlinks(this.roots[scope], file)
    if (!fs.existsSync(file)) return { version: MANIFEST_VERSION, artifacts: [] }
    const parsed = JSON.parse(fs.readFileSync(file, 'utf8'))
    if (parsed.version !== MANIFEST_VERSION || !Array.isArray(parsed.artifacts)) throw new Error(`Unsupported integration manifest: ${file}`)
    return parsed
  }

  atomicWrite(file, content) {
    this.assertNoSymlinks(path.parse(file).root === file ? file : this.rootFor(file), file)
    fs.mkdirSync(path.dirname(file), { recursive: true })
    const tmp = path.join(path.dirname(file), `.${path.basename(file)}.${process.pid}.${crypto.randomBytes(6).toString('hex')}.tmp`)
    try {
      fs.writeFileSync(tmp, content, { encoding: 'utf8', mode: 0o600 })
      fs.renameSync(tmp, file)
    } finally {
      if (fs.existsSync(tmp)) fs.unlinkSync(tmp)
    }
  }

  rootFor(file) {
    const found = Object.values(this.roots).find(root => {
      const rel = path.relative(root, file)
      return rel && !rel.startsWith(`..${path.sep}`) && !path.isAbsolute(rel)
    })
    if (!found) throw new Error(`Path is outside integration roots: ${file}`)
    return found
  }

  saveManifest(scope, manifest) {
    manifest.artifacts.sort((a, b) => a.destination.localeCompare(b.destination))
    for (const artifact of manifest.artifacts) artifact.claims.sort((a, b) => claimKey(a).localeCompare(claimKey(b)))
    this.atomicWrite(this.manifestPath(scope), `${JSON.stringify(manifest, null, 2)}\n`)
  }

  inspect(plans) {
    const manifests = new Map()
    return plans.map(plan => {
      const { scope } = plan.claim
      if (!manifests.has(scope)) manifests.set(scope, this.loadManifest(scope))
      const file = this.resolve(scope, plan.destination)
      const record = manifests.get(scope).artifacts.find(entry => entry.destination === plan.destination)
      const owned = record && record.claims.some(claim => claimKey(claim) === claimKey(plan.claim))
      if (!fs.existsSync(file)) return { ...plan, state: 'not-installed', managed: Boolean(owned) }
      const actual = sha256(fs.readFileSync(file))
      if (!owned) return { ...plan, state: 'modified', managed: false }
      if (actual !== record.sha256) return { ...plan, state: 'modified', managed: true }
      const desired = sha256(plan.content)
      const state = desired === actual && record.catalog_version === plan.catalogVersion ? 'current' : 'outdated'
      return { ...plan, state, managed: true }
    })
  }

  install(plans) { return this.mutate('install', plans, false) }
  update(plans, { force = false } = {}) { return this.mutate('update', plans, force) }
  uninstall(plans, { force = false } = {}) { return this.mutate('uninstall', plans, force) }

  mutate(operation, plans, force) {
    const snapshots = new Map()
    const manifests = new Map()
    const scopes = [...new Set(plans.map(plan => plan.claim.scope))]
    for (const scope of scopes) {
      manifests.set(scope, this.loadManifest(scope))
      this.snapshot(snapshots, this.manifestPath(scope))
    }
    for (const plan of plans) {
      const file = this.resolve(plan.claim.scope, plan.destination)
      this.snapshot(snapshots, file)
    }
    try {
      for (const plan of plans) this.apply(operation, plan, manifests.get(plan.claim.scope), force)
      for (const scope of scopes) this.saveManifest(scope, manifests.get(scope))
    } catch (error) {
      this.rollback(snapshots)
      throw error
    }
    return this.inspect(plans)
  }

  apply(operation, plan, manifest, force) {
    const file = this.resolve(plan.claim.scope, plan.destination)
    let record = manifest.artifacts.find(entry => entry.destination === plan.destination)
    const key = claimKey(plan.claim)
    const owned = record && record.claims.some(claim => claimKey(claim) === key)
    const exists = fs.existsSync(file)
    const actual = exists ? sha256(fs.readFileSync(file)) : ''
    const desired = sha256(plan.content)

    if (operation === 'uninstall') {
      if (!owned) return
      if (exists && actual !== record.sha256 && !force) throw new Error(`Refusing to remove modified file without --force: ${plan.destination}`)
      record.claims = record.claims.filter(claim => claimKey(claim) !== key)
      if (record.claims.length === 0) {
        if (exists) fs.unlinkSync(file)
        manifest.artifacts = manifest.artifacts.filter(entry => entry !== record)
        this.cleanEmpty(path.dirname(file), this.roots[plan.claim.scope])
      }
      return
    }

    if (!record) {
      if (exists) {
        const legacy = actual === desired || (plan.legacyHashes || []).includes(actual)
        if (!legacy) throw new Error(`Refusing to adopt unmanaged file: ${plan.destination}`)
        record = { destination: plan.destination, sha256: actual, catalog_version: plan.catalogVersion, claims: [] }
        manifest.artifacts.push(record)
        record.claims.push({ ...plan.claim, support_level: actual === desired ? plan.supportLevel : 'legacy' })
        if (operation === 'update' && actual !== desired) {
          this.atomicWrite(file, plan.content)
          record.sha256 = desired
          record.catalog_version = plan.catalogVersion
          record.claims[0].support_level = plan.supportLevel
        }
        return
      }
      this.atomicWrite(file, plan.content)
      manifest.artifacts.push({ destination: plan.destination, sha256: desired, catalog_version: plan.catalogVersion, claims: [{ ...plan.claim, support_level: plan.supportLevel }] })
      return
    }

    const modified = exists && actual !== record.sha256
    if (operation === 'install' && modified) throw new Error(`Refusing to claim modified managed file: ${plan.destination}`)
    if (!owned) record.claims.push({ ...plan.claim, support_level: plan.supportLevel })
    if (operation === 'install') {
      if (!exists) {
        this.atomicWrite(file, plan.content)
        record.sha256 = desired
        record.catalog_version = plan.catalogVersion
      }
      return
    }
    if (modified && !force) throw new Error(`Refusing to overwrite modified file without --force: ${plan.destination}`)
    if (!exists || actual !== desired) this.atomicWrite(file, plan.content)
    record.sha256 = desired
    record.catalog_version = plan.catalogVersion
    const claim = record.claims.find(entry => claimKey(entry) === key)
    claim.support_level = plan.supportLevel
  }

  snapshot(snapshots, file) {
    if (snapshots.has(file)) return
    snapshots.set(file, fs.existsSync(file) ? fs.readFileSync(file) : null)
  }

  rollback(snapshots) {
    for (const [file, content] of [...snapshots.entries()].reverse()) {
      try {
        if (content === null) {
          if (fs.existsSync(file) && !fs.lstatSync(file).isDirectory()) fs.unlinkSync(file)
        } else {
          fs.mkdirSync(path.dirname(file), { recursive: true })
          const tmp = `${file}.rollback-${process.pid}`
          fs.writeFileSync(tmp, content)
          fs.renameSync(tmp, file)
        }
      } catch { /* retain the original error */ }
    }
  }

  cleanEmpty(dir, root) {
    let current = dir
    while (current !== root) {
      const rel = path.relative(root, current)
      if (!rel || rel.startsWith(`..${path.sep}`) || path.isAbsolute(rel)) break
      if (!fs.existsSync(current) || fs.lstatSync(current).isSymbolicLink()) break
      if (fs.readdirSync(current).length) break
      fs.rmdirSync(current)
      current = path.dirname(current)
    }
  }
}

module.exports = { IntegrationManager, sha256, claimKey }
