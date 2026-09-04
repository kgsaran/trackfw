'use strict'

// ML-4A de ROADMAP-2026-09-03-fechar-os-grupos-de-falha-de-windows-por-causa-raiz:
// guarda de plataforma MEDIDA para os asserts de bit de execucao.
//
// ## Por que existe
//
// Em NTFS o bit de execucao nao existe: fs.statSync(p).mode & 0o111 devolve 0 para TODO
// arquivo, inclusive imediatamente depois de fs.chmodSync(p, 0o755). Um assert "o artefato
// gerado e executavel" nao mede o gerador ali -- mede uma propriedade que o sistema de
// arquivos nao tem, e reprova sempre.
//
// A decisao de suprimir a checagem apenas onde o bit nao e representavel esta tomada em
// vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md:
// o bit NUNCA foi discriminante em NTFS, e o WSL (kernel Linux, ext4) continua coberto
// porque ali o bit e representavel de verdade.
//
// ## Detecao pela CONDICAO, nao por process.platform
//
// A sonda MEDE o filesystem em vez de inferir da plataforma. Ela recebe o DIRETORIO onde o
// teste escreve, porque e esse o filesystem sob medicao -- um probe no os.tmpdir() global
// mediria outro volume.
//
// ## Nao e skip, de proposito
//
// O teste inteiro continua rodando; so o assert do bit e suprimido, e a supressao NOMEIA a
// garantia que deixou de ser verificada.
//
// Este arquivo NAO casa com o glob `tests/*.test.js` do `npm test`, entao nao vira suite.

const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')

// execBitRepresentavel(dir) -> boolean
// Neste sistema de arquivos, um arquivo criado em `dir` e levado a 0o755 por chmodSync
// passa a ter (mode & 0o111) !== 0?
//
// Nao ha catch: se a propria sonda nao puder rodar, o erro sobe. "Nao consegui medir" nao
// pode virar supressao silenciosa dos asserts.
function execBitRepresentavel (dir) {
  const base = dir || os.tmpdir()
  const p = path.join(base, `trackfw-execbit-probe-${process.pid}-${Math.random().toString(36).slice(2)}.sh`)
  try {
    fs.writeFileSync(p, '#!/bin/sh\nexit 0\n', { mode: 0o644 })
    fs.chmodSync(p, 0o755)
    return (fs.statSync(p).mode & 0o111) !== 0
  } finally {
    fs.rmSync(p, { force: true })
  }
}

// execBitRepresentavelPara(artefato) -> boolean
// Forma usada nos call sites: mede o filesystem do arquivo que esta sendo verificado.
function execBitRepresentavelPara (artefato) {
  return execBitRepresentavel(path.dirname(artefato))
}

// execBitNaoExercitado(artefato): registra, com tag grepavel, QUAL garantia deixou de ser
// verificada e por que.
function execBitNaoExercitado (artefato) {
  console.error(
    `EXEC-BIT-NAO-EXERCITADO: ${artefato} -- garantia NAO verificada: "o artefato foi criado com o bit ` +
    'de execucao (0755)". Este sistema de arquivos devolve (mode & 0o111) === 0 mesmo apos ' +
    'fs.chmodSync(0o755) (NTFS nao representa o bit). O restante do teste continua medindo. ' +
    'Decisao: vault/notes/goos-guard-e-do-binario-nao-do-host-wsl-continua-protegido-2026-09-01.md'
  )
}

module.exports = { execBitRepresentavel, execBitRepresentavelPara, execBitNaoExercitado }
