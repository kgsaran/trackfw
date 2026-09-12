// 🔴 LOCALIZAÇÃO TRANSITÓRIA — auditoria do arquiteto, 2026-09-12.
// Este teste vive em `npm/tests/` para rodar no `make quality` de hoje, mas ele testa o
// PROTÓTIPO (`prototype/packages/`). Duas coisas o tornam órfão no futuro:
//   1. o `prototype/` é descartável e sai antes do PR de produção;
//   2. o AC6 da REQ da v8 remove `npm/tests/` inteiro — ele testa a reimplementação.
// Quando o protótipo virar produção, este teste vai junto com o shim, não com a suíte antiga.
// Se você chegou aqui porque ele quebrou depois de o `prototype/` sumir: mova-o, não o apague —
// os dois braços dele são o que impede a colisão de `bin` e a quebra de resolução de voltarem.

'use strict'

// shim_packaging.test.js — falsificação de duas propriedades simultâneas do empacotamento
// do shim npm (Opção D, ML-1C/ML-1D do ROADMAP-2026-09-12-validar-um-binario-muitos-canais).
//
// CONTEXTO DA REGRESSÃO (2026-09-12):
//   Ares reportou colisão: shim e @trackfw-bin/win32-x64 ambos declaravam `bin: {trackfw: ...}`,
//   brigando por node_modules/.bin/trackfw de forma dependente da ordem de install (não-determinista).
//   KG removeu `bin` dos 6 pacotes de plataforma. Isso quebrou a resolução: o shim lia `pkgJson.bin`
//   e falhava com "has no bin entry" (P15: shim_exit=1 em win32/x64).
//   A correção: shim resolve por subcaminho conhecido (`bin/trackfw` ou `bin/trackfw.exe`) calculado
//   a partir de `os.platform()`, sem ler `package.json.bin`.
//
// POR QUE DOIS BRAÇOS JUNTOS:
//   A falha foi trocar um estado ruim por outro silenciosamente — nenhum teste forçava a escolha certa.
//   Arm A sozinho prova ausência de `bin`; Arm B sozinho prova que a resolução funciona HOJE.
//   Juntos, eles provam que os dois estados ruins são simultaneamente impossíveis:
//     - Arm A reprova se `bin` retornar (colisão possível novamente)
//     - Arm B reprova se a resolução quebrar (regressão de 2026-09-12)
//   O binding entre os dois: Arm B verifica exatamente o subcaminho que Arm A prova que não está
//   no `bin`. Alterar o layout dos pacotes (ex.: mover o binário para `dist/`) quebra Arm B
//   mesmo com Arm A passando.

const test = require('node:test')
const assert = require('node:assert/strict')
const path = require('node:path')
const fs = require('node:fs')
const os = require('node:os')
const { spawnSync } = require('node:child_process')

const PROTO_PACKAGES = path.resolve(__dirname, '../../prototype/packages')
const SHIM_JS = path.join(PROTO_PACKAGES, 'trackfw-shim/bin/trackfw.js')

const PLATFORMS = [
  'darwin-arm64',
  'darwin-x64',
  'linux-arm64',
  'linux-x64',
  'win32-arm64',
  'win32-x64',
]

// Subcaminho que o shim calcula para cada plataforma (lógica espelhada do shim).
// win32-* → bin/trackfw.exe · todos os outros → bin/trackfw
function expectedSubpath(platform) {
  return platform.startsWith('win32-') ? 'bin/trackfw.exe' : 'bin/trackfw'
}

// ── Arm A: nenhum pacote de plataforma declara `bin` ────────────────────────
//
// Reprova se `bin` retornar → colisão em node_modules/.bin/trackfw possível novamente.
// A colisão é nondeterminista: qual arquivo vence depende da ordem de npm install,
// e o loser fica inacessível sem mensagem de erro.

test('Arm A — platform packages têm sem campo bin (sem colisão em node_modules/.bin/)', () => {
  for (const plat of PLATFORMS) {
    const pkgJsonPath = path.join(PROTO_PACKAGES, `trackfw-${plat}/package.json`)
    const pkgJson = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'))
    assert.strictEqual(
      pkgJson.bin,
      undefined,
      `@trackfw-bin/${plat}: campo "bin" não deve existir — causa colisão nondeterminista em node_modules/.bin/trackfw`
    )
  }
})

// ── Arm B (estático): subcaminho correto declarado em `files` ────────────────
//
// Para cada pacote de plataforma, o subcaminho que o shim computaria (`bin/trackfw` ou
// `bin/trackfw.exe`) DEVE estar no array `files`. Isso vincula Arm A e Arm B:
//   - Arm A prova que `bin` está ausente.
//   - Arm B (estático) prova que o arquivo existe pelo caminho que o shim espera.
// Se alguém mover o binário para `dist/trackfw` e atualizar `files`, Arm B reprova mesmo
// com Arm A verde — e vice-versa.

test('Arm B (estático) — cada pacote de plataforma declara o subcaminho correto em files', () => {
  for (const plat of PLATFORMS) {
    const pkgJsonPath = path.join(PROTO_PACKAGES, `trackfw-${plat}/package.json`)
    const pkgJson = JSON.parse(fs.readFileSync(pkgJsonPath, 'utf8'))
    const sub = expectedSubpath(plat)
    assert.ok(
      Array.isArray(pkgJson.files) && pkgJson.files.includes(sub),
      `@trackfw-bin/${plat}: "files" deve incluir "${sub}" (subcaminho que o shim usa para resolver o binário)`
    )
  }
})

// ── Arm B (em disco): arquivo existe no pacote da plataforma atual ────────────
//
// Para a plataforma onde este teste está rodando, o binário declarado em `files`
// DEVE existir no disco. Prova que o `npm pack` incluiria o arquivo real.
// (Plataformas cruzadas — ex.: win32 rodando no macOS — são puladas: sem cross-binary.)

test('Arm B (disco) — binário existe no pacote da plataforma atual', () => {
  const platform = os.platform()
  const arch = os.arch()
  const currentPlat = `${platform}-${arch}`

  if (!PLATFORMS.includes(currentPlat)) {
    // plataforma não reconhecida (ex.: win32-ia32) — pular sem falhar
    return
  }

  const sub = expectedSubpath(currentPlat)
  const binPath = path.join(PROTO_PACKAGES, `trackfw-${currentPlat}`, sub)

  assert.ok(
    fs.existsSync(binPath),
    `binário ausente em prototype/packages/trackfw-${currentPlat}/${sub} — npm pack não incluiria o executável`
  )
})

// ── Arm B (runtime): shim resolve sem ler `bin` ──────────────────────────────
//
// Afirma: o shim resolve corretamente dado um pacote de plataforma SEM campo `bin`.
// Falsificação negativa: um revert para `pkgJson.bin` emite "has no bin entry" → reprova.
// Falsificação positiva: um subpath errado emite "binary not found" → reprova.
//
// Portabilidade:
//   Não asserta exit 0 — scripts de shell não são executáveis como PE no Windows
//   (CreateProcess valida o cabeçalho, não a extensão). Asserta que stderr NÃO contém
//   as duas mensagens de falha de resolução conhecidas. O shim chama spawnSync(fakeBin, ...),
//   que pode falhar por não ser um executável válido na plataforma — isso é esperado e
//   irrelevante para a resolução.
//
// NODE_PATH: aponta para o fake node_modules. O shim usa require.resolve relativo ao
//   arquivo do shim; sem node_modules reais próximos, NODE_PATH é a única rota.

test('Arm B (runtime) — shim resolve via subcaminho sem campo bin', () => {
  const platform = os.platform()
  const arch = os.arch()
  const currentPlat = `${platform}-${arch}`

  const tmpBase = fs.mkdtempSync(path.join(os.tmpdir(), 'shim-falsify-'))
  try {
    // Monta fake node_modules/@trackfw-bin/<platform>/ com package.json SEM `bin`
    const fakePkgDir = path.join(tmpBase, 'node_modules', '@trackfw-bin', currentPlat)
    const fakeBinDir = path.join(fakePkgDir, 'bin')
    fs.mkdirSync(fakeBinDir, { recursive: true })

    // package.json sem campo `bin` — é exatamente o estado que causou a regressão
    const fakePkgJson = {
      name: `@trackfw-bin/${currentPlat}`,
      version: '0.0.1',
      // `bin` propositalmente ausente — o shim não deve precisar deste campo
    }
    fs.writeFileSync(path.join(fakePkgDir, 'package.json'), JSON.stringify(fakePkgJson))

    // Binário mínimo: script de shell (não é PE válido — mas isso é irrelevante para a
    // resolução, que é o que estamos testando). O shim chama spawnSync(fakeBin, args),
    // que pode falhar na execução; só queremos que a RESOLUÇÃO não falhe.
    const binaryName = platform === 'win32' ? 'trackfw.exe' : 'trackfw'
    const fakeBin = path.join(fakeBinDir, binaryName)
    const script = platform === 'win32' ? '@echo fake-bin\r\n' : '#!/bin/sh\necho fake-bin\n'
    fs.writeFileSync(fakeBin, script, { mode: 0o755 })

    // Invoca o shim com NODE_PATH → fake node_modules
    const result = spawnSync(
      process.execPath,
      [SHIM_JS],               // sem args adicionais — o shim vai falhar na execução do fake binary
      {
        env: { ...process.env, NODE_PATH: path.join(tmpBase, 'node_modules') },
        encoding: 'utf8',
        timeout: 5000,
      }
    )

    const stderr = result.stderr || ''

    // Arm B reprova se qualquer uma das duas mensagens de falha de RESOLUÇÃO aparecer:
    //   1. "has no bin entry"  → revert para pkgJson.bin (regressão de 2026-09-12)
    //   2. "binary not found"  → subpath errado ou arquivo ausente no pacote
    assert.ok(
      !stderr.includes('has no bin entry'),
      `shim ainda lê pkgJson.bin — regressão de 2026-09-12 ativa:\n${stderr}`
    )
    assert.ok(
      !stderr.includes('binary not found in'),
      `shim não achou o binário pelo subcaminho — package layout divergiu do que o shim espera:\n${stderr}`
    )
  } finally {
    fs.rmSync(tmpBase, { recursive: true, force: true })
  }
})
