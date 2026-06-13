'use strict'
const fs = require('fs')
const path = require('path')

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

/**
 * toSlug — converte string em slug kebab-case lowercase.
 * @param {string} s
 * @returns {string}
 */
function toSlug(s) {
  return s.toLowerCase().replace(/ /g, '-')
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
  const date = new Date().toISOString().slice(0, 10)
  const filename = `${reqDir}/REQ-${date}-${slug}.md`

  const motivationSection = content.motivation || '<!-- Why is this requirement needed? What problem does it solve? -->'
  const criteriaSection = content.criteria || '- [ ]\n- [ ]'
  const linkedADRSection = ''
  const linkedRoadmapSection = ''

  const dependsOnADRs = content.dependsOnADRs || []

  // Linha de status — inclui contador de ADRs bloqueantes quando presente
  let statusLine = `> Date: ${date} | Status: Open`
  if (dependsOnADRs.length > 0) {
    statusLine = `> Date: ${date} | Status: Open | Blocked by ADRs: ${dependsOnADRs.length}`
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

  const body = `# REQ: ${content.title}

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

module.exports = { listREQs, parseREQStatus, newREQ, PROBES_CATALOG, detectDomains }
