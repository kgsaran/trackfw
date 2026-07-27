'use strict'
const fs = require('fs')
const path = require('path')
const { localDateISO } = require('./date')

/**
 * listREQs — lista arquivos .md em dir, imprimindo filename e status (coluna 60 chars).
 * Extrai status da linha `> Date: ... | Status: ...`.
 * Se dir não existe ou vazio: imprime "No REQs found in <dir>".
 */
function listREQs(dir) {
  let files = []
  try {
    files = fs.readdirSync(dir).filter(f => f.endsWith('.md'))
  } catch (_) {
    // dir não existe
  }

  if (files.length === 0) {
    console.log(`No REQs found in ${dir}`)
    return
  }

  for (const filename of files) {
    const filepath = path.join(dir, filename)
    const status = parseREQStatus(filepath)
    console.log(`${filename.padEnd(60)} ${status}`)
  }
}

/**
 * parseREQStatus — extrai o status da linha `> Date: ... | Status: ...` de um arquivo REQ.
 * Status termina no próximo " |" ou fim da linha.
 */
function parseREQStatus(filepath) {
  let content
  try {
    content = fs.readFileSync(filepath, 'utf8')
  } catch (_) {
    return 'unknown'
  }

  for (const line of content.split('\n')) {
    const idx = line.indexOf('| Status: ')
    if (idx >= 0) {
      let rest = line.slice(idx + '| Status: '.length)
      const pipeIdx = rest.indexOf(' |')
      if (pipeIdx >= 0) {
        rest = rest.slice(0, pipeIdx)
      }
      rest = rest.replace(/[\s>|]+$/, '')
      return rest.trim() || 'unknown'
    }
  }
  return 'unknown'
}

function rewriteREQStatus(source, status) {
  if (!source.startsWith('---\n')) return { content: source, changed: false }
  const end = source.slice(4).indexOf('\n---')
  if (end < 0) return { content: source, changed: false }

  let changed = false
  const frontmatter = source.slice(4, 4 + end)
  let rest = source.slice(4 + end)
  const lines = frontmatter.split('\n')

  for (let i = 0; i < lines.length; i++) {
    const idx = lines[i].indexOf(':')
    if (idx < 0) continue
    const rawKey = lines[i].slice(0, idx)
    if (rawKey.trim() !== 'status') continue
    const value = lines[i].slice(idx + 1).trim()
    const quoted = value.length >= 2 && value.startsWith('"') && value.endsWith('"')
    const newLine = quoted ? `${rawKey}: "${status}"` : `${rawKey}: ${status}`
    if (lines[i] !== newLine) {
      lines[i] = newLine
      changed = true
    }
    break
  }

  if (rest.length > 4) {
    const bodyLines = rest.slice(4).split('\n')
    const marker = '| Status: '
    for (let i = 0; i < bodyLines.length; i++) {
      if (bodyLines[i].trim().startsWith('## ')) break
      const idx = bodyLines[i].indexOf(marker)
      if (idx < 0) continue
      const prefix = bodyLines[i].slice(0, idx + marker.length)
      const after = bodyLines[i].slice(idx + marker.length)
      const pipeIdx = after.indexOf(' |')
      const suffix = pipeIdx >= 0 ? after.slice(pipeIdx) : ''
      const newLine = `${prefix}${status}${suffix}`
      if (bodyLines[i] !== newLine) {
        bodyLines[i] = newLine
        changed = true
        rest = '\n---' + bodyLines.join('\n')
      }
      break
    }
  }

  if (!changed) return { content: source, changed: false }
  return { content: `---\n${lines.join('\n')}${rest}`, changed: true }
}

function findREQ(name, reqDir) {
  let files = []
  try {
    files = fs.readdirSync(reqDir).filter(f => f.endsWith('.md'))
  } catch (e) {
    throw new Error(`reading REQ dir: ${e.message}`)
  }
  const lower = name.toLowerCase()
  const found = files.find(f => f.toLowerCase().includes(lower))
  if (!found) throw new Error(`REQ "${name}" not found in ${reqDir}`)
  return path.join(reqDir, found)
}

function moveREQ(name, status) {
  if (!String(status || '').trim()) throw new Error('status is required')
  const reqDir = require('../config').load().reqDir
  const filepath = findREQ(name, reqDir)
  const source = fs.readFileSync(filepath, 'utf8')
  const result = rewriteREQStatus(source, status)
  if (!result.changed) {
    throw new Error(`REQ "${path.basename(filepath)}" has no frontmatter status/header Status to update`)
  }
  fs.writeFileSync(filepath, result.content, 'utf8')
  console.log(`✓ updated ${path.basename(filepath)} status → ${status}`)
}

/**
 * toSlug — converte string em slug kebab-case lowercase.
 * @param {string} s
 * @returns {string}
 */
function toSlug(s) {
  // NFKD normalization + remove combining marks (diacríticos) + lowercase + non-alphanumeric → hífen
  return s
    .normalize('NFKD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
}

/**
 * newREQ — cria docs/req/REQ-YYYY-MM-DD-<slug>.md.
 * @param {{ title: string, motivation?: string, criteria?: string, dependsOnADRs?: string[] }} content
 * @returns {Promise<void>}
 */
async function newREQ(content) {
  const reqDir = require('../config').load().reqDir
  fs.mkdirSync(reqDir, { recursive: true })

  const slug = toSlug(content.title)
  const date = localDateISO()
  const filename = `${reqDir}/REQ-${date}-${slug}.md`

  const motivationSection = content.motivation || '<!-- Why is this requirement needed? What problem does it solve? -->'
  const criteriaSection = content.criteria || '- [ ]\n- [ ]'
  const linkedADRSection = ''
  const linkedRoadmapSection = ''

  const dependsOnADRs = content.dependsOnADRs || []

  // Linha de status — inclui contador de ADRs bloqueantes quando presente
  let statusLine = `> Date: ${date} | Status: Open\n| Linear Issue: \n| Jira Issue: `
  if (dependsOnADRs.length > 0) {
    statusLine = `> Date: ${date} | Status: Open | Blocked by ADRs: ${dependsOnADRs.length}\n| Linear Issue: \n| Jira Issue: `
  }

  // Seção "Blocked by ADRs"
  let blockedSection
  if (dependsOnADRs.length === 0) {
    blockedSection = '<!-- none -->'
  } else {
    const lines = ['<!-- ADRs in Draft status that must be Accepted before a roadmap can be created -->']
    for (const adr of dependsOnADRs) {
      lines.push(`- ${adr} (Draft)`)
    }
    blockedSection = lines.join('\n')
  }

  const body = `---
status: Open
date: ${date}
author: ""
adr: ""
roadmap: ""
---

# REQ: ${content.title}

${statusLine}

## Motivation
${motivationSection}

## Acceptance Criteria
${criteriaSection}

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: ${linkedADRSection}

## Blocked by ADRs
${blockedSection}

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: ${linkedRoadmapSection}
`

  fs.writeFileSync(filename, body, 'utf8')
  console.log(`created ${filename}`)
}

/**
 * PROBES_CATALOG — catálogo de domínios técnicos detectáveis (porte exato do Go).
 */
const PROBES_CATALOG = [
  {
    domain: 'authentication',
    keywords: ['login', 'auth', 'senha', 'password', 'sso', 'jwt', 'session', 'token', 'autenticação', 'autenticar'],
    questions: [
      {
        text: 'How will users authenticate?',
        options: [
          { label: 'Local login (email + password)', decided: true, adrSlug: '' },
          { label: 'SSO (Google, Azure AD, Okta...)', decided: false, adrSlug: 'sso-provider' },
          { label: 'Both (local + SSO)', decided: false, adrSlug: 'authentication-strategy' },
          { label: 'Not decided yet', decided: false, adrSlug: 'authentication-strategy' },
        ],
      },
      {
        text: 'How will sessions be managed?',
        options: [
          { label: 'JWT (stateless)', decided: true, adrSlug: '' },
          { label: 'Server-side sessions (cookies)', decided: true, adrSlug: '' },
          { label: 'Not decided yet', decided: false, adrSlug: 'session-management' },
        ],
      },
    ],
  },
  {
    domain: 'ui',
    keywords: ['tela', 'screen', 'ui', 'frontend', 'componente', 'component', 'design', 'layout', 'interface'],
    questions: [
      {
        text: 'Is there an existing UI framework or design system?',
        options: [
          { label: 'Yes, already chosen', decided: true, adrSlug: '' },
          { label: 'No, need to choose a UI framework', decided: false, adrSlug: 'ui-framework' },
          { label: 'Not relevant for this REQ', decided: true, adrSlug: '' },
        ],
      },
    ],
  },
  {
    domain: 'persistence',
    keywords: ['banco', 'database', 'db', 'tabela', 'table', 'migração', 'migration', 'modelo', 'model', 'persistência', 'persist'],
    questions: [
      {
        text: 'Which database engine will be used?',
        options: [
          { label: 'Already decided', decided: true, adrSlug: '' },
          { label: 'Not decided yet', decided: false, adrSlug: 'database-engine' },
        ],
      },
    ],
  },
  {
    domain: 'api',
    keywords: ['api', 'endpoint', 'rest', 'grpc', 'graphql', 'rota', 'route', 'http'],
    questions: [
      {
        text: 'Which API protocol will be used?',
        options: [
          { label: 'REST (already decided)', decided: true, adrSlug: '' },
          { label: 'gRPC (already decided)', decided: true, adrSlug: '' },
          { label: 'GraphQL (already decided)', decided: true, adrSlug: '' },
          { label: 'Not decided yet', decided: false, adrSlug: 'api-protocol' },
        ],
      },
    ],
  },
  {
    domain: 'deploy',
    keywords: ['deploy', 'cloud', 'container', 'kubernetes', 'k8s', 'docker', 'infra', 'aws', 'gcp', 'azure'],
    questions: [
      {
        text: 'Is the deployment infrastructure already defined?',
        options: [
          { label: 'Yes, fully defined', decided: true, adrSlug: '' },
          { label: 'Cloud provider not decided', decided: false, adrSlug: 'cloud-provider' },
          { label: 'Container strategy not decided', decided: false, adrSlug: 'container-strategy' },
        ],
      },
    ],
  },
  {
    domain: 'events',
    keywords: ['kafka', 'fila', 'queue', 'notificação', 'notification', 'evento', 'event', 'pubsub', 'pub/sub', 'broker', 'sqs', 'redis'],
    questions: [
      {
        text: 'Which event broker will be used?',
        options: [
          { label: 'Already decided', decided: true, adrSlug: '' },
          { label: 'Not decided yet', decided: false, adrSlug: 'event-broker' },
        ],
      },
    ],
  },
]

/**
 * detectDomains — retorna probes cujos keywords aparecem na intention (case-insensitive).
 * @param {string} intention
 * @returns {Array}
 */
function detectDomains(intention) {
  const lower = intention.toLowerCase()
  return PROBES_CATALOG.filter(probe =>
    probe.keywords.some(kw => lower.includes(kw.toLowerCase()))
  )
}

module.exports = { listREQs, parseREQStatus, rewriteREQStatus, moveREQ, newREQ, PROBES_CATALOG, detectDomains, localDateISO, toSlug }
