'use strict'

const fs = require('fs')
const path = require('path')

/**
 * Converte uma string em slug: lowercase + espaços → hifens.
 * @param {string} s
 * @returns {string}
 */
function toSlug(s) {
  return s.toLowerCase().replace(/ /g, '-')
}

/**
 * Converte slug com hifens em Title Case.
 * Ex: "authentication-strategy" → "Authentication Strategy"
 * @param {string} slug
 * @returns {string}
 */
function slugToTitle(slug) {
  return slug
    .split('-')
    .map((w) => (w.length > 0 ? w[0].toUpperCase() + w.slice(1) : w))
    .join(' ')
}

/**
 * Retorna a data atual no formato YYYY-MM-DD.
 * @returns {string}
 */
function today() {
  return new Date().toISOString().slice(0, 10)
}

/**
 * Cria um novo ADR em docs/adr/ADR-YYYY-MM-DD-<slug>.md.
 * Campos vazios recebem placeholder HTML.
 * @param {{ title: string, context?: string, decision?: string, consequences?: string, alternatives?: string }} content
 * @returns {Promise<void>}
 */
async function newADR(content) {
  const adrDir = require('../config').load().adrDirs[0]
  fs.mkdirSync(adrDir, { recursive: true })

  const slug = toSlug(content.title)
  const date = today()
  const filename = `${adrDir}/ADR-${date}-${slug}.md`

  const contextSection = content.context || '<!-- What is the situation that motivates this decision? -->'
  const decisionSection = content.decision || '<!-- What was decided? -->'
  const consequencesSection = content.consequences || '<!-- What are the positive and negative consequences of this decision? -->'
  const alternativesSection = content.alternatives || '<!-- What other options were evaluated and why were they rejected? -->'

  const body = `---
status: Proposed
date: ${date}
author: ""
---

# ADR: ${content.title}

> Date: ${date} | Status: Proposed

## Context
${contextSection}

## Decision
${decisionSection}

## Consequences
${consequencesSection}

## Alternatives Considered
${alternativesSection}
`

  fs.writeFileSync(filename, body, 'utf8')
  console.log(`created ${filename}`)
}

/**
 * Lista todos os ADRs (.md) em dir, imprimindo filename e status (coluna 60 chars).
 * @param {string} dir
 * @returns {Promise<void>}
 */
async function listADRs(dir) {
  if (!fs.existsSync(dir)) {
    console.log(`No ADRs found in ${dir}`)
    return
  }

  const files = fs.readdirSync(dir).filter((f) => f.endsWith('.md')).sort()

  if (files.length === 0) {
    console.log(`No ADRs found in ${dir}`)
    return
  }

  for (const file of files) {
    const filepath = path.join(dir, file)
    const status = parseADRStatus(filepath)
    const padded = file.padEnd(60)
    console.log(`${padded} ${status}`)
  }
}

/**
 * Extrai o status de um arquivo ADR markdown.
 * Procura pela linha "> Date: ... | Status: ..."
 * @param {string} filepath
 * @returns {string}
 */
function parseADRStatus(filepath) {
  try {
    const content = fs.readFileSync(filepath, 'utf8')
    const lines = content.split('\n')
    for (const line of lines) {
      const idx = line.indexOf('| Status: ')
      if (idx >= 0) {
        let rest = line.slice(idx + '| Status: '.length)
        rest = rest.replace(/[ >|]+$/, '').trim()
        return rest
      }
    }
  } catch (_) {
    // ignorar erros de leitura
  }
  return 'unknown'
}

/**
 * Cria um ADR com Status: Draft a partir de um slug.
 * Idempotente: se já existe ADR-*-<slug>.md, pula e imprime mensagem.
 * @param {string} slug
 * @returns {Promise<string>} basename do arquivo criado
 */
async function newADRDraft(slug) {
  const adrDir = require('../config').load().adrDirs[0]
  fs.mkdirSync(adrDir, { recursive: true })

  // Verificar idempotência: buscar arquivo existente com o mesmo slug
  const existing = fs.existsSync(adrDir)
    ? fs.readdirSync(adrDir).find((f) => f.match(new RegExp(`^ADR-.*-${slug}\\.md$`)))
    : null

  if (existing) {
    console.log(`skipped ${existing} (already exists)`)
    return existing
  }

  const date = today()
  const filename = `ADR-${date}-${slug}.md`
  const filepath = path.join(adrDir, filename)
  const title = slugToTitle(slug)

  const body = `---
status: Draft
date: ${date}
author: ""
---

# ADR: ${title}

> Date: ${date} | Status: Draft

## Context
<!-- What is the situation that motivates this decision? -->

## Decision
<!-- What was decided? -->

## Consequences
<!-- What are the positive and negative consequences of this decision? -->

## Alternatives Considered
<!-- What other options were evaluated and why were they rejected? -->
`

  fs.writeFileSync(filepath, body, 'utf8')
  console.log(`created ${filename}`)
  return filename
}

module.exports = { newADR, listADRs, newADRDraft, toSlug }
