'use strict'

// changelog — parsing e extração de seções do CHANGELOG.md no formato
// Keep a Changelog (https://keepachangelog.com/en/1.1.0/).
// Port 1:1 de internal/changelog/changelog.go — manter em sincronia.

const fs = require('fs')
const path = require('path')

// sectionHeaderRE casa cabeçalhos de seção no formato "## [x.y.z] - YYYY-MM-DD"
// ou "## [Unreleased]" (sem data).
const sectionHeaderRE = /^## \[([^\]]+)\](?: - (\d{4}-\d{2}-\d{2}))?/

/**
 * parseSections — separa o conteúdo de um CHANGELOG.md em seções, uma por
 * cabeçalho "## [...]" encontrado. Texto antes da primeira seção (título do
 * arquivo, preâmbulo) é descartado.
 * @param {string} content
 * @returns {Array<{version: string, date: string, body: string}>}
 */
function parseSections(content) {
  const lines = content.split('\n')

  const sections = []
  let current = null
  let bodyLines = []

  const flush = () => {
    if (current !== null) {
      current.body = bodyLines.join('\n')
      sections.push(current)
    }
  }

  for (const line of lines) {
    const m = sectionHeaderRE.exec(line)
    if (m !== null) {
      flush()
      current = { version: m[1], date: m[2] || '', body: '' }
      bodyLines = []
      continue
    }
    if (current !== null) {
      bodyLines.push(line)
    }
  }
  flush()

  return sections
}

/**
 * firstSection — retorna a primeira seção da lista.
 * Erro se a lista vier vazia.
 * @param {Array<{version: string, date: string, body: string}>} sections
 * @returns {{version: string, date: string, body: string}}
 */
function firstSection(sections) {
  if (sections.length === 0) {
    throw new Error('CHANGELOG.md has no version sections')
  }
  return sections[0]
}

/**
 * findVersion — busca a seção com version igual ao argumento, normalizando um
 * prefixo "v"/"V" opcional no argumento do usuário antes de comparar.
 * @param {Array<{version: string, date: string, body: string}>} sections
 * @param {string} version
 * @returns {{version: string, date: string, body: string}}
 */
function findVersion(sections, version) {
  let normalized = version
  if (normalized.length > 0 && (normalized[0] === 'v' || normalized[0] === 'V')) {
    normalized = normalized.slice(1)
  }
  for (const s of sections) {
    if (s.version === normalized || s.version === version) {
      return s
    }
  }
  throw new Error(`version "${version}" not found in CHANGELOG.md`)
}

/**
 * formatSection — reconstrói o texto formatado de uma seção, reproduzindo o
 * cabeçalho original.
 * @param {{version: string, date: string, body: string}} section
 * @returns {string}
 */
function formatSection(section) {
  const dateSuffix = section.date !== '' ? ` - ${section.date}` : ''
  const body = section.body.replace(/^\n+/, '')
  return `## [${section.version}]${dateSuffix}\n\n${body.replace(/\n+$/, '') + '\n'}`
}

/**
 * read — lê o CHANGELOG.md na raiz informada.
 * @param {string} root
 * @returns {string}
 */
function read(root) {
  const filePath = path.join(root, 'CHANGELOG.md')
  try {
    return fs.readFileSync(filePath, 'utf8')
  } catch (err) {
    if (err.code === 'ENOENT') {
      throw new Error('CHANGELOG.md not found — nothing to show')
    }
    throw err
  }
}

module.exports = { parseSections, firstSection, findVersion, formatSection, read }
