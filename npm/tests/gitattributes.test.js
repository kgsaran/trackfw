'use strict'

// ML-1A (ROADMAP-2026-09-02-gitattributes-com-merge-union-para-o-trackfw-log-nos-3-clis):
// os três ramos de generateGitAttributes. O gate scripts/check-artifact-parity.sh
// cobre só o ramo de CRIAÇÃO cross-runtime; o de APPEND só existe aqui e nos
// equivalentes Go/Python.

const test = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

const { generateGitAttributes, GITATTRIBUTES_BLOCK } = require('../src/generators/init')

function tmpProject() {
  return fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-gitattributes-'))
}

test('generateGitAttributes cria o arquivo quando ausente e é idempotente', () => {
  const dir = tmpProject()
  try {
    generateGitAttributes(dir)
    const target = path.join(dir, '.gitattributes')
    const first = fs.readFileSync(target, 'utf8')
    assert.equal(first, GITATTRIBUTES_BLOCK)
    generateGitAttributes(dir)
    assert.equal(fs.readFileSync(target, 'utf8'), first, 'init duas vezes não pode duplicar a regra')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('generateGitAttributes faz append sem grudar na última linha de arquivo sem newline final', () => {
  const dir = tmpProject()
  try {
    const target = path.join(dir, '.gitattributes')
    fs.writeFileSync(target, '* text=auto', 'utf8')
    generateGitAttributes(dir)
    const want = '* text=auto\n' + GITATTRIBUTES_BLOCK
    assert.equal(fs.readFileSync(target, 'utf8'), want)
    generateGitAttributes(dir)
    assert.equal(fs.readFileSync(target, 'utf8'), want, 'segunda execução duplicou a regra')
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('generateGitAttributes não sobrescreve regra preexistente com outro espaçamento', () => {
  const dir = tmpProject()
  try {
    const target = path.join(dir, '.gitattributes')
    const existing = '.trackfw-log  merge=union\n'
    fs.writeFileSync(target, existing, 'utf8')
    generateGitAttributes(dir)
    assert.equal(fs.readFileSync(target, 'utf8'), existing)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('linha comentada não conta como regra existente', () => {
  const dir = tmpProject()
  try {
    const target = path.join(dir, '.gitattributes')
    fs.writeFileSync(target, '# .trackfw-log merge=union\n', 'utf8')
    generateGitAttributes(dir)
    assert.equal(fs.readFileSync(target, 'utf8'), '# .trackfw-log merge=union\n' + GITATTRIBUTES_BLOCK)
  } finally {
    fs.rmSync(dir, { recursive: true, force: true })
  }
})

test('bloco gerado é igual ao .gitattributes versionado na raiz do repositório', () => {
  const versioned = fs.readFileSync(path.join(__dirname, '..', '..', '.gitattributes'), 'utf8')
  assert.equal(versioned, GITATTRIBUTES_BLOCK)
})
