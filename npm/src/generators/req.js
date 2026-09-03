'use strict'
const fs = require('fs')
const path = require('path')
const { localDateISO } = require('./date')
const roadmapGen = require('./roadmap')
const config = require('../config')
const { resolveAgentNamespaces, resolveReqFiles, reqWriteDir } = require('../validator')

const VALID_STATES = roadmapGen.VALID_STATES
const STATE_ORDER = roadmapGen.STATE_ORDER

/**
 * listREQFiles — NÃO reimplementa a descoberta: delega ao ponto único de leitura do validador
 * (resolveReqFiles — ADR-2026-09-03, D3/D4), o mesmo consumido pelas regras de validate. Duas
 * noções de layout no mesmo runtime foram a causa do defeito da REQ-2026-08-30.
 * @param {object} cfg — config completo (ver npm/src/config)
 * @returns {string[]} paths completos, união dos 4 layouts, deduplicados e ordenados
 */
function listREQFiles(cfg) {
  return resolveReqFiles(cfg)
}

/**
 * listREQs — lista REQs (recursivo nos 3 layouts), imprimindo filename e status (coluna 60 chars).
 * Extrai status da linha `> Date: ... | Status: ...`.
 * Se nenhum REQ encontrado: imprime "No REQs found in <reqDir>".
 * @param {object} cfg — config completo (ver npm/src/config)
 */
function listREQs(cfg) {
  const files = listREQFiles(cfg)

  if (files.length === 0) {
    console.log(`No REQs found in ${cfg.reqDir}`)
    return
  }

  for (const filepath of files) {
    const filename = path.basename(filepath)
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

/**
 * findREQ — busca recursiva nos 3 layouts (flat → por-estado → by_agent), retornando o primeiro
 * path cujo basename contém `name` (case-insensitive).
 * @param {string} name
 * @param {object} cfg — config completo (ver npm/src/config)
 * @returns {string} path completo
 */
function findREQ(name, cfg) {
  const files = listREQFiles(cfg)
  const lower = name.toLowerCase()
  const found = files.find(f => path.basename(f).toLowerCase().includes(lower))
  if (!found) throw new Error(`REQ "${name}" not found in ${cfg.reqDir}`)
  return found
}

/**
 * appendREQTransitionLog — append em <reqDir>/.trackfw-log, mesmo formato de
 * appendTransitionLog (roadmap.js), em arquivo de log separado (escopo de REQ, não roadmap).
 */
function appendREQTransitionLog(cfg, basename, fromState, toState) {
  const now = new Date()
  const yyyy = now.getFullYear()
  const mm = String(now.getMonth() + 1).padStart(2, '0')
  const dd = String(now.getDate()).padStart(2, '0')
  const hh = String(now.getHours()).padStart(2, '0')
  const min = String(now.getMinutes()).padStart(2, '0')
  const timestamp = `${yyyy}-${mm}-${dd} ${hh}:${min}`
  const line = `${timestamp}  ${basename.padEnd(50)}  ${fromState} → ${toState}\n`

  try {
    const lp = path.join(cfg.reqDir, '.trackfw-log')
    fs.mkdirSync(path.dirname(lp), { recursive: true })
    fs.appendFileSync(lp, line, 'utf8')
  } catch (_) { /* best-effort, mesmo padrão do roadmap */ }
}

/**
 * moveREQ — reescreve status: e, condicionalmente, move fisicamente o arquivo
 * (ADR-2026-08-04, decisão D3):
 * - REQ solta em `reqDir/` → modo in-place (comportamento legado: só reescreve status, sem mover).
 * - REQ já organizada em `reqDir/<estado>/` ou `reqDir/<agente>/<estado>/` → move fisicamente
 *   para o novo estado, preservando o layout (por-estado ou by_agent), e loga a transição.
 */
function moveREQ(name, status) {
  if (!String(status || '').trim()) throw new Error('status is required')
  const cfg = require('../config').load()
  const filepath = findREQ(name, cfg)
  const source = fs.readFileSync(filepath, 'utf8')
  const result = rewriteREQStatus(source, status)
  if (!result.changed) {
    throw new Error(`REQ "${path.basename(filepath)}" has no frontmatter status/header Status to update`)
  }

  const basename = path.basename(filepath)
  const parentDir = path.dirname(filepath)
  const grandparentDir = path.dirname(parentDir)
  const greatGrandparentDir = path.dirname(grandparentDir)
  const reqDirAbs = path.resolve(cfg.reqDir)
  const fromState = path.basename(parentDir)

  if (path.resolve(parentDir) === reqDirAbs) {
    // modo in-place — REQ solta em reqDir/, comportamento legado (sem mover)
    fs.writeFileSync(filepath, result.content, 'utf8')
    console.log(`✓ updated ${basename} status → ${status}`)
    return
  }

  if (!VALID_STATES.includes(status)) {
    throw new Error(`invalid state "${status}" — valid states: ${VALID_STATES.join(', ')}`)
  }

  let targetDir = null
  let logBasename = basename

  if (path.resolve(grandparentDir) === reqDirAbs && VALID_STATES.includes(fromState)) {
    // layout por-estado — reqDir/<estado>/
    targetDir = path.join(cfg.reqDir, status)
  } else if (VALID_STATES.includes(fromState) && path.resolve(greatGrandparentDir) === reqDirAbs) {
    // layout by_agent — reqDir/<agente>/<estado>/
    const agent = path.basename(grandparentDir)
    targetDir = path.join(cfg.reqDir, agent, status)
    logBasename = `${agent}/${basename}`
  }

  if (!targetDir) {
    // layout não reconhecido — fallback seguro para in-place (não inventa destino)
    fs.writeFileSync(filepath, result.content, 'utf8')
    console.log(`✓ updated ${basename} status → ${status}`)
    return
  }

  fs.mkdirSync(targetDir, { recursive: true })
  const dst = path.join(targetDir, basename)
  fs.writeFileSync(dst, result.content, 'utf8')
  if (path.resolve(dst) !== path.resolve(filepath)) {
    fs.unlinkSync(filepath)
  }
  appendREQTransitionLog(cfg, logBasename, fromState, status)
  console.log(`✓ moved ${basename} → ${targetDir}`)
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
  // Ponto único de decisão de caminho de ESCRITA (ADR-2026-09-03, D2/D4): by_agent grava no
  // canônico req_dir/<agente>/; flat grava em req_dir/ — o mesmo ponto que alimenta a união de leitura.
  const cfg = require('../config').load()
  const reqDir = reqWriteDir(cfg) || cfg.reqDir
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

module.exports = { listREQs, listREQFiles, findREQ, parseREQStatus, rewriteREQStatus, moveREQ, newREQ, PROBES_CATALOG, detectDomains, localDateISO, toSlug }
