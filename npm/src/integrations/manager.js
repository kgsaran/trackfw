'use strict'

const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { frontmatterName } = require('./render')
const { tildeify } = require('../lib/update-engine')

const SCHEMA_VERSION = 1
const sha256 = content => crypto.createHash('sha256').update(content).digest('hex')
// claimKey includes origin (ADR-2026-08-15 D11) so a catalog claim and a
// third-party claim that happen to share target/surface/scope/kind/item are
// never treated as the same ownership record — mirrors Go's Claim struct
// equality (`==`), field-by-field, which already includes Origin
// (internal/integrations/manager.go's claimOwned/appendClaim/removeClaim).
const claimKey = claim => [claim.target, claim.surface, claim.scope, claim.kind, claim.item, claim.origin || ''].join('\u0000')
// cleanClaim omits origin entirely when falsy/absent, mirroring Go's
// `json:"origin,omitempty"` — a catalog claim (origin "") must serialize
// with NO "origin" key at all, byte-identical to a manifest written before
// this field existed (retrocompatibility, D11).
const cleanClaim = claim => {
  const out = { target: claim.target, surface: claim.surface, scope: claim.scope, kind: claim.kind, item: claim.item }
  if (claim.origin) out.origin = claim.origin
  return out
}

class IntegrationManager {
  constructor({ projectRoot = process.cwd(), homeRoot = os.homedir() } = {}, { onSkip } = {}) {
    this.roots = { project: path.resolve(projectRoot), global: path.resolve(homeRoot) }
    this.onSkip = onSkip
    // _afterManifestPersist, when set, runs immediately after manifests are
    // persisted and before artifact bytes are written during install/update
    // (never during uninstall, which is intentionally not inverted — see the
    // comment in mutate()). It exists only so tests can simulate an
    // interruption exactly at the ADR-2026-08-18 ordering seam. Production
    // code never assigns it. Mirrors internal/integrations/manager.go's
    // afterManifestPersist package var.
    this._afterManifestPersist = undefined
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
      if (!destination || path.posix.normalize(destination) !== destination || destination === '.' || destination.startsWith('../')) throw new Error(`Unsafe destination: ${destination}`)
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
      // registered reports whether the manifest has ANY entry for this
      // destination, regardless of claim ownership — unlike managed, which
      // additionally requires this exact claim to own that entry. doctor
      // (ML-2A) needs this distinction: a destination registered under a
      // *different* claim must never be reported as an "unregistered
      // write" — that would be the dominant false-positive doctor exists to
      // avoid. Additive field; mirrors Inspection.Registered
      // (internal/integrations/manager.go).
      const registered = Boolean(record)
      if (!fs.existsSync(file)) return { ...plan, destination: file, state: 'not-installed', managed, registered }
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
      return { ...plan, destination: file, state, managed, registered }
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
    const skippedFiles = new Set()
    for (const item of resolved) {
      const desired = sha256(item.plan.content)
      if (operation !== 'uninstall' && desiredByFile.has(item.file) && desiredByFile.get(item.file) !== desired) throw new Error(`Conflicting content planned for: ${item.file}`)
      desiredByFile.set(item.file, desired)
      const skip = this.preflight(operation, item, manifests.get(item.plan.claim.scope), force)
      if (skip) {
        skippedFiles.add(item.file)
        if (typeof this.onSkip === 'function') {
          // O manager compõe a linha completa e passa tilde-abreviado como
          // destination (contrato: docs/cli-parity.md, "Valor de cada
          // parâmetro — pinado"). A remediação é derivada de
          // plan.claim.scope por artefato, nunca de inferência sobre o
          // caminho renderizado.
          const abbrev = this.tildeAbbrev(item.file, item.plan.claim.scope)
          const remediation = item.plan.claim.scope === 'global' ? 'trackfw update harness' : 'trackfw update'
          const reason = `warning: skipping outdated artifact ${abbrev}; run '${remediation}' to refresh it`
          this.onSkip(abbrev, reason)
        }
      }
    }
    const active = resolved.filter(item => !skippedFiles.has(item.file))
    const snapshots = new Map()
    for (const item of active) this.snapshot(snapshots, item.file)
    for (const scope of manifests.keys()) this.snapshot(snapshots, this.manifestPath(scope))
    const saveManifests = () => {
      for (const [scope, manifest] of [...manifests].sort(([a], [b]) => a.localeCompare(b))) this.saveManifest(scope, manifest)
    }
    try {
      // ADR-2026-08-18: install/update persist the manifest before writing
      // artifact bytes (self-healing direction on interruption); uninstall
      // keeps the original artifacts-before-manifest order. The two are
      // deliberately NOT symmetric.
      if (operation === 'uninstall') {
        // Uninstall is not inverted. Removing bytes first and persisting the
        // manifest last means an interruption leaves the manifest still
        // declaring an artifact whose file is now absent — inspectResolved
        // resolves that as 'not-installed', the same self-healing direction
        // install/update get from the inversion below. Inverting uninstall
        // the same way (drop the manifest entry, then remove bytes) would
        // instead leave, on interruption, a file on disk whose content still
        // matches the catalog template but with no manifest entry at all —
        // reported as an orphaned state='current'/managed=false artifact
        // that looks legitimate and that nothing detects or repairs
        // automatically. That is exactly the "disk ahead of manifest" bad
        // direction the ADR exists to eliminate, so uninstall must not be
        // simetrized with install/update here. Mirrors the comment in
        // internal/integrations/manager.go:mutate.
        for (const item of active) this.applyUninstall(item, manifests.get(item.plan.claim.scope))
        saveManifests()
      } else {
        // Install/Update: compute the manifest update for every active item
        // in memory (no bytes touched yet), persist all manifests, and only
        // then write the artifact bytes.
        const pendingWrites = []
        for (const item of active) {
          const write = this.planArtifactWrite(operation, item, manifests.get(item.plan.claim.scope), force)
          if (write) pendingWrites.push(write)
        }
        saveManifests()
        if (typeof this._afterManifestPersist === 'function') this._afterManifestPersist()
        for (const write of pendingWrites) this.atomicWrite(write.file, write.content, write.mode)
      }
    } catch (error) {
      this.rollback(snapshots)
      throw error
    }
    return this.inspect(plans)
  }

  preflight(operation, { plan, file }, manifest, force) {
    if (operation !== 'uninstall') this.detectNameCollision(plan, file, force)
    const status = this.inspectResolved(plan, file, manifest)
    const record = manifest.artifacts[file]
    const owned = Boolean(record && record.claims.some(claim => claimKey(claim) === claimKey(plan.claim)))
    if (operation === 'install') {
      if (status.state === 'modified' && !force) throw new Error(`Artifact is modified; use --force: ${file}`)
      if (status.state === 'outdated' && owned && !force) return true // skip: bytes preserved, batch continues
    } else if (operation === 'update') {
      if (!owned && status.state === 'modified') throw new Error(this.unmanagedArtifactError(file, plan.claim))
      if (status.state === 'modified' && !force) throw new Error(`Artifact is modified; use --force: ${file}`)
    } else if (operation === 'uninstall' && owned && status.state === 'modified' && !force) {
      throw new Error(`Artifact is modified; use --force: ${file}`)
    }
    return false
  }

  // detectNameCollision protege contra dois artefatos de agente gerenciados
  // distintos declarando o mesmo "name" de frontmatter dentro do mesmo
  // diretório de destino (ADR ADR-2026-07-25-identidade-personalizavel-de-
  // agentes, seção D4). Isso importa porque, com identidades customizáveis,
  // dois item.id diferentes poderiam resolver para o mesmo nome renderizado
  // (ex.: dois agentes ambos apontados para o slug "zeus"), e algumas
  // surfaces usam esse campo name para descoberta de agentes.
  //
  // Limitação: a varredura só inspeciona irmãos ".md", porque é o único
  // formato onde temos uma forma barata e sem dependências (frontmatterName)
  // de ler de volta o name declarado a partir de bytes já renderizados.
  // Artefatos JSON (cli-agent-json/agent-json) e TOML (custom-agent-toml)
  // não são varridos para colisões. Espelha
  // internal/integrations/manager.go:detectNameCollision.
  detectNameCollision(plan, file, force) {
    if (plan.claim.kind !== 'agents') return
    if (path.extname(file) !== '.md') return
    const desiredName = frontmatterName(plan.content)
    if (!desiredName) return
    const directory = path.dirname(file)
    let entries
    try {
      entries = fs.readdirSync(directory, { withFileTypes: true })
    } catch (err) {
      if (err.code === 'ENOENT') return
      throw new Error(`scan ${directory} for name collisions: ${err.message}`)
    }
    for (const entry of entries) {
      if (entry.isDirectory() || path.extname(entry.name) !== '.md') continue
      const candidate = path.join(directory, entry.name)
      if (candidate === file) continue
      let data
      try {
        data = fs.readFileSync(candidate)
      } catch {
        continue
      }
      const candidateName = frontmatterName(data)
      if (!candidateName || candidateName !== desiredName) continue
      if (force) {
        process.stderr.write(`aviso: ${candidate} declara o mesmo name ${desiredName} que ${file}; prosseguindo por --force\n`)
        continue
      }
      throw new Error(`artifact ${file} declares name ${desiredName} which collides with existing file ${candidate}`)
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

  // applyUninstall removes ownership of one artifact from the manifest, and —
  // once no claim remains — the artifact's bytes and any empty ancestor
  // directories it managed. It mutates disk directly (not deferred), because
  // uninstall deliberately keeps the pre-ADR-2026-08-18 ordering: see the
  // comment in mutate() for why this is not simetrized with
  // planArtifactWrite. Mirrors internal/integrations/manager.go:applyUninstall.
  applyUninstall({ plan, file }, manifest) {
    const record = manifest.artifacts[file]
    const key = claimKey(plan.claim)
    const owned = Boolean(record && record.claims.some(claim => claimKey(claim) === key))
    if (!owned) return
    record.claims = record.claims.filter(claim => claimKey(claim) !== key)
    if (record.claims.length) return
    if (fs.existsSync(file)) fs.unlinkSync(file)
    delete manifest.artifacts[file]
    this.cleanEmpty(path.dirname(file), this.roots[plan.claim.scope])
  }

  // planArtifactWrite computes the manifest update for one install/update
  // item entirely in memory — it never touches the artifact's bytes on disk.
  // The caller (mutate) persists every manifest in the batch first, and only
  // then applies the returned pending write (ADR-2026-08-18). Returns null
  // when no byte write is needed.
  //
  // The manifest values it stores are deliberately *optimistic* when a write
  // is pending: sha256/catalog_version describe the content this call is
  // about to write, not what is currently on disk. If interrupted before the
  // pending write lands, the manifest already declares the target state and
  // inspectResolved resolves the (absent or stale) file to
  // 'not-installed'/'modified', both self-repairable by a later
  // install/update, never modified+unowned ("unmanaged"). Mirrors
  // internal/integrations/manager.go:planArtifactWrite.
  planArtifactWrite(operation, { plan, file }, manifest, force) {
    let record = manifest.artifacts[file]
    const key = claimKey(plan.claim)
    const owned = Boolean(record && record.claims.some(claim => claimKey(claim) === key))

    const exists = fs.existsSync(file)
    let actual = exists ? sha256(fs.readFileSync(file)) : ''
    const desired = sha256(plan.content)
    const knownLegacy = (plan.legacyHashes || []).includes(actual)
    let writeDesired = !exists
    if (exists && !owned) writeDesired = (operation === 'update' && actual !== desired) || (force && actual !== desired)
    else if (exists && owned) writeDesired = actual !== desired
    if (!record) record = { destination: file, sha256: '', catalog_version: '', claims: [] }
    let pending = null
    if (writeDesired) {
      // Optimistic: bytes have not moved yet, but the manifest must already
      // describe the content we are about to write (see doc comment above).
      actual = desired
      pending = { file, content: plan.content, mode: 0o644 }
    } else if (exists && !owned && actual !== desired && !knownLegacy && !force) {
      // Defense-in-depth: preflight already rejects this exact case for
      // 'update' (unconditionally) and for 'install' without --force (any
      // state === 'modified' is blocked before an active item ever reaches
      // planArtifactWrite()). This branch is therefore not reachable via
      // install/update today, but stays as a second line of defense in case
      // preflight's guard is ever loosened — hence the identical remediation
      // text, so a user who somehow hits it still gets the same actionable
      // message. No manifest mutation happens on this path. Mirrors
      // internal/integrations/manager.go:planArtifactWrite.
      throw new Error(this.unmanagedArtifactError(file, plan.claim))
    }
    if (!record.claims.some(claim => claimKey(claim) === key)) record.claims.push(cleanClaim(plan.claim))
    record.sha256 = actual
    record.catalog_version = actual === desired ? plan.catalogVersion : 'legacy'
    manifest.artifacts[file] = record
    return pending
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

  // unmanagedArtifactError — mensagem de erro para update/uninstall (ou,
  // defensivamente, install) recusando bytes que o trackfw não escreveu.
  // Nomeia o remédio — trackfw did not write these bytes, então a única forma
  // segura de trazer o artefato sob gestão é `<kind> install --force`, que
  // explicitamente autoriza adotar/substituir conteúdo unmanaged — com as
  // flags exatas para reproduzir o claim deste plano (item, target, scope),
  // prontas para copiar e colar. Espelha
  // internal/integrations/manager.go:unmanagedArtifactError — fonte canônica
  // do texto, byte a byte idêntica nos 3 CLIs.
  unmanagedArtifactError(file, claim) {
    return `unmanaged artifact "${file}" does not match a trackfw template — trackfw did not write these bytes.\nAdopt it with: trackfw ${claim.kind} install --force --items ${claim.item} --targets ${claim.target} --scope ${claim.scope}`
  }

  // tildeAbbrev — retorna o caminho de exibição abreviado para uso em mensagens
  // de aviso de skip. Espelha internal/integrations/manager.go:tildeAbbrev.
  // - scope global: substitui homeRoot por '~' (via tildeify, com salvaguarda
  //   de barra dupla corrigida no ML-6H).
  // - scope de projeto: caminho relativo ao projectRoot, sem './' prefixo.
  tildeAbbrev(file, scope) {
    if (scope === 'global') return tildeify(this.roots.global, file)
    return path.relative(this.roots.project, file)
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
