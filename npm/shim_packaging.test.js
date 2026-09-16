// shim_packaging.test.js — falsificação das propriedades do empacotamento do shim npm.
//
// ML-1B, ROADMAP-2026-09-12-v8-um-binario-muitos-canais.
// Ported from prototype (ML-1C/ML-1D, validar-branch) for production shim at npm/bin/trackfw.js.
//
// Scope — o que este arquivo afirma e o que pertence a outro ML:
//
//   Arms A e B-estático (sem `bin` no package.json dos pacotes de plataforma) pertencem
//   ao ML-1A: são propriedades do GERADOR de manifests (os package.json de plataforma não
//   existem no repositório — são artefato de build). Afirmá-los aqui seria circular: o teste
//   verificaria exatamente o que foi commitado, sem tocar o que vai a produção.
//   Ref: ML-1A, AC3, issue #338, ADR-2026-09-12.
//
//   Este arquivo afirma:
//     Arm B (runtime) — o shim resolve corretamente COM pacote de plataforma sem `bin`.
//     Arm C (ausência) — o shim aborta com exit 1 e nomeia a plataforma, sem MODULE_NOT_FOUND.
//
// As duas propriedades são provadas JUNTAS: B-runtime exige que o `bin` esteja ausente
// do pacote fake (condição inicial do braço) e ainda assim o shim resolve. C-ausência exige
// que o erro de plataforma ausente não vaze como MODULE_NOT_FOUND.
//
// Se alguém reverter o shim para ler `pkgJson.bin`, B-runtime passa "has no bin entry"
// para stderr e reprova. Se alguém remover o erro amigável, C-ausência deixa de encontrar
// a string da plataforma e reprova.
//
// Quando a Wave 3 remover npm/tests/ (AC6), mova este arquivo junto com o shim — não apague.

'use strict'

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const fs = require('node:fs')
const os = require('node:os')
const { spawnSync } = require('node:child_process')

// O shim de produção — o objeto que este arquivo testa.
const SHIM_JS = path.resolve(__dirname, '../bin/trackfw.js')

// ── Arm B (runtime): shim resolve via subcaminho sem campo bin ───────────────
//
// Afirma: o shim resolve o binário de plataforma dado um pacote SEM campo `bin`.
// Falsificação negativa: reverter para `pkgJson.bin` emite "has no bin entry" → reprova.
// Falsificação positiva: subpath errado emite "binary not found" → reprova.
//
// Este braço prova as DUAS propriedades do ML-1B simultaneamente:
//   (1) pacotes de plataforma sem `bin` (AC13)
//   (2) shim ainda resolve (AC1)
// sem precisar de arquivos commitados de plataforma — usa tmpdir.
//
// Portabilidade: não asserta exit 0 porque o fake binary não é um PE válido no Windows.
// O que importa é que a RESOLUÇÃO não falhe; a execução pode retornar erro de OS.

test('Arm B (runtime) — shim resolve via subcaminho sem campo bin', () => {
  const platform = os.platform()
  const arch = os.arch()
  const currentPlat = `${platform}-${arch}`

  const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'shim-prod-'))
  try {
    // Monta fake node_modules/@trackfw-bin/<platform>/ com package.json SEM `bin`
    const fakePkgDir = path.join(tmpBase, 'node_modules', '@trackfw-bin', currentPlat)
    const fakeBinDir = path.join(fakePkgDir, 'bin')
    fs.mkdirSync(fakeBinDir, { recursive: true })

    // package.json sem campo `bin` — é exatamente o estado que causou a regressão de 2026-09-12
    const fakePkgJson = {
      name: `@trackfw-bin/${currentPlat}`,
      version: '0.0.1',
      // `bin` propositalmente ausente — o shim não deve precisar deste campo
    }
    fs.writeFileSync(path.join(fakePkgDir, 'package.json'), JSON.stringify(fakePkgJson))

    // Binário fake: pode não ser executável válido — só precisamos que a resolução funcione.
    const binaryName = platform === 'win32' ? 'trackfw.exe' : 'trackfw'
    const fakeBin = path.join(fakeBinDir, binaryName)
    const script = platform === 'win32' ? '@echo fake-bin\r\n' : '#!/bin/sh\necho fake-bin\n'
    fs.writeFileSync(fakeBin, script, { mode: 0o755 })

    const result = spawnSync(
      process.execPath,
      [SHIM_JS],
      {
        env: { ...process.env, NODE_PATH: path.join(tmpBase, 'node_modules') },
        encoding: 'utf8',
        timeout: 5000,
      }
    )

    const stderr = result.stderr || ''

    // Afirma: "sem campo `bin` no pacote de plataforma, o shim resolve pelo subcaminho calculado."
    // Reprova se qualquer mensagem de falha de RESOLUÇÃO aparecer:
    assert.ok(
      !stderr.includes('has no bin entry'),
      `shim ainda lê pkgJson.bin — regressão de 2026-09-12 ativa:\n${stderr}`
    )
    assert.ok(
      !stderr.includes('binary not found in'),
      `shim não achou o binário pelo subcaminho — package layout divergiu:\n${stderr}`
    )
    assert.ok(
      !stderr.includes('is not installed'),
      `shim falhou na resolução do pacote de plataforma:\n${stderr}`
    )
  } finally {
    fs.rmSync(tmpBase, { recursive: true, force: true })
  }
})

// ── Arm C (ausência): sem pacote de plataforma → exit 1, plataforma nomeada ──
//
// Afirma: quando o pacote de plataforma não está instalado, o shim:
//   (a) encerra com exit 1
//   (b) menciona a plataforma atual (ex.: "darwin/arm64") no stderr
//   (c) NÃO vaza MODULE_NOT_FOUND
//
// (c) é o discriminante — sem ele, o arm passa em qualquer `require()` que exploda,
// incluindo regressões onde o shim não trata o erro.
//
// Afirma: "sem pacote de plataforma instalado, o shim aborta nomeando a plataforma, sem MODULE_NOT_FOUND."

test('Arm C (ausência) — sem pacote de plataforma: exit 1, plataforma nomeada, sem MODULE_NOT_FOUND', () => {
  const platform = os.platform()
  const arch = os.arch()

  // tmpdir vazio — nenhum @trackfw-bin/* instalado
  const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'shim-absent-'))
  try {
    const emptyModules = path.join(tmpBase, 'node_modules')
    fs.mkdirSync(emptyModules, { recursive: true })

    const result = spawnSync(
      process.execPath,
      [SHIM_JS],
      {
        env: { ...process.env, NODE_PATH: emptyModules },
        encoding: 'utf8',
        timeout: 5000,
      }
    )

    const stderr = result.stderr || ''

    // (a) exit 1 — não deve sair silenciosamente
    assert.strictEqual(
      result.status,
      1,
      `shim saiu com status ${result.status} em vez de 1 quando pacote ausente`
    )

    // (b) stderr menciona a plataforma — "darwin/arm64", "linux/x64", etc.
    const platformStr = `${platform}/${arch}`
    assert.ok(
      stderr.includes(platformStr),
      `stderr não nomeia a plataforma "${platformStr}" — usuário não sabe por que falhou:\n${stderr}`
    )

    // (c) MODULE_NOT_FOUND não deve vazar — seria o erro bruto do require(), não a mensagem amigável
    assert.ok(
      !stderr.includes('MODULE_NOT_FOUND'),
      `stderr contém MODULE_NOT_FOUND — shim não tratou o erro de resolução:\n${stderr}`
    )
  } finally {
    fs.rmSync(tmpBase, { recursive: true, force: true })
  }
})
