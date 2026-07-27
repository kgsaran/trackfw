'use strict'

const fs = require('fs')
const path = require('path')
const { localDateISO } = require('./date')

const VAULT_DIR = 'vault/notes'
const INDEX_FILE = 'vault/notes/index.md'

/**
 * Converte título em slug kebab-case lowercase sem acentos.
 * Reutiliza a mesma lógica do adr.js (toSlug).
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
 * Retorna a data LOCAL atual no formato YYYY-MM-DD.
 * Delega para localDateISO() — usa getDate/getMonth/getFullYear (hora local),
 * não toISOString (UTC).
 * @returns {string}
 */
function today() {
  return localDateISO()
}

/**
 * Cria uma nova nota em vault/notes/<slug>-YYYY-MM-DD.md e linka no index.md.
 * Idempotente: falha com erro claro se a nota já existir.
 * @param {string} title
 * @param {string} [cwd]
 */
function newNote(title, cwd) {
  const root = cwd || process.cwd()
  const vaultDir = path.join(root, VAULT_DIR)
  fs.mkdirSync(vaultDir, { recursive: true })

  const slug = toSlug(title)
  const date = today()
  const filename = `${slug}-${date}.md`
  const notePath = path.join(vaultDir, filename)

  if (fs.existsSync(notePath)) {
    throw new Error(`nota "${filename}" já existe — não sobrescrita`)
  }

  const body = `---
title: "${title}"
tags: []
date: ${date}
related: []
---

# ${title}

## Problem

<!-- Descreva o problema ou situação que motivou esta nota. -->

## Root cause

<!-- Qual foi a causa raiz identificada? -->

## Solution

<!-- Como foi resolvido ou mitigado? O que deve ser feito? -->
`

  fs.writeFileSync(notePath, body, 'utf8')
  appendNoteToIndex(filename, root)
  console.log(`created ${path.join(VAULT_DIR, filename)}`)
}

/**
 * Acrescenta link para filename no vault/notes/index.md.
 * Cria o index.md se não existir. Não duplica se já estiver linkado.
 * @param {string} filename
 * @param {string} [cwd]
 */
function appendNoteToIndex(filename, cwd) {
  const root = cwd || process.cwd()
  const indexPath = path.join(root, INDEX_FILE)

  if (!fs.existsSync(indexPath)) {
    const initial = `# Vault de Conhecimento

> Ponto de entrada de conhecimento do projeto para agentes e pessoas.

## Índice

`
    fs.writeFileSync(indexPath, initial, 'utf8')
  }

  const content = fs.readFileSync(indexPath, 'utf8')
  const nameWithoutExt = filename.replace(/\.md$/, '')

  // Verifica se já linkado (link markdown ou wikilink)
  if (
    content.includes(`(${filename})`) ||
    content.includes(`[[${nameWithoutExt}]]`) ||
    content.includes(`[[${filename}]]`)
  ) {
    return
  }

  const link = `- [${nameWithoutExt}](${filename})\n`
  fs.appendFileSync(indexPath, link, 'utf8')
}

/**
 * Retorna todos os arquivos .md em vault/notes/ exceto index.md.
 * Retorna [] se o diretório não existir.
 * @param {string} [cwd]
 * @returns {string[]}
 */
function noteFiles(cwd) {
  const root = cwd || process.cwd()
  const vaultDir = path.join(root, VAULT_DIR)
  if (!fs.existsSync(vaultDir)) return []
  return fs
    .readdirSync(vaultDir)
    .filter((f) => f.endsWith('.md') && f !== 'index.md')
}

/**
 * Retorna true se o index.md referencia filename.
 * @param {string} filename
 * @param {string} [cwd]
 * @returns {boolean}
 */
function indexContains(filename, cwd) {
  const root = cwd || process.cwd()
  const indexPath = path.join(root, INDEX_FILE)
  if (!fs.existsSync(indexPath)) return false
  const content = fs.readFileSync(indexPath, 'utf8')
  const nameWithoutExt = filename.replace(/\.md$/, '')
  return (
    content.includes(`(${filename})`) ||
    content.includes(`[[${nameWithoutExt}]]`) ||
    content.includes(`[[${filename}]]`)
  )
}

module.exports = { newNote, appendNoteToIndex, noteFiles, indexContains, toSlug, today }
