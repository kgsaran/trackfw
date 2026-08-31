'use strict'

// probe.js — sonda sob demanda (ADR-2026-08-30, decisao 3 / AC5 / AC6 / AC9),
// braco Node. Espelha scripts/windows-repro/go/probe.go: imprime o valor
// BRUTO que o SO devolveu para cada pergunta e para ai — sem veredito, sem
// comparacao contra "esperado". Quem le decide o que significa.
//
// NAO E checks.js (camada 2, mapeada aos 11 itens da issue #216, COM
// veredito REPRODUCED/ABSENT) — ver windows-probe.yml para a distincao.
// checks.js/checks.py nunca devem importar as funcoes deste arquivo.
//
// Introduzido por ROADMAP-2026-08-30-sonda-mede-junction-nos-3-runtimes-e-
// a-pergunta-7-volta-a-responder.md, ML-1A, para responder AC3/AC5 da
// REQ-2026-08-30 (junction em Node nunca havia sido medida).
//
// Executado via `node scripts/windows-repro/node/probe.js <subcomando>`.

const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')

// Imprime o diretorio temporario que de fato foi resolvido ao lado de
// RUNNER_TEMP — achado do ML-0A (hades-tf): fs.mkdtempSync(os.tmpdir())
// resolve via %TEMP%, que NAO e RUNNER_TEMP. Medir em vez de presumir.
function printTempDirInfo(tmp) {
  process.stdout.write(`tempdir_resolvido=${tmp} runner_temp=${process.env.RUNNER_TEMP || ''}\n`)
}

// Cria a junction via `cmd /c mklink /J` — MESMO mecanismo que probe.go e
// probe.py usam (não `fs.symlinkSync(..., 'junction')`, que embora produza o
// mesmo reparse tag IO_REPARSE_TAG_MOUNT_POINT, é escrito pelo libuv com um
// REPARSE_DATA_BUFFER próprio — SubstituteName/PrintName podem divergir do
// que mklink.exe grava, e são exatamente os campos que readlink()/LinkType
// leem. Usar uma via diferente por runtime confundiria "o Lstat de cada
// runtime diverge" com "o objeto medido é diferente" — o dado que esta
// sonda existe para produzir tem que vir do MESMO objeto nos 3 runtimes.
function mklinkJunction(junction, targetDir) {
  return spawnSync('cmd', ['/c', 'mklink', '/J', junction, targetDir], { encoding: 'utf8' })
}

function lstatOrError(p) {
  try {
    return fs.lstatSync(p)
  } catch (err) {
    return err
  }
}

function printLstat(label, target, statOrErr) {
  if (statOrErr instanceof Error) {
    process.stdout.write(`${label} path=${target} err=${statOrErr.message}\n`)
    return
  }
  const info = statOrErr
  process.stdout.write(
    `${label} path=${target} isSymbolicLink=${info.isSymbolicLink()} isDirectory=${info.isDirectory()} isFile=${info.isFile()}\n`
  )
}

// Pergunta (referencia) — lstatSync sobre um arquivo comum, para comparacao
// com symlink e junction abaixo. Mesmo papel de cmdLstatCommon em probe.go.
function cmdLstatCommon() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-probe-common-'))
  const file = path.join(tmp, 'common.txt')
  fs.writeFileSync(file, 'trackfw-probe-target\n')
  printLstat('lstat-common', file, lstatOrError(file))
  fs.rmSync(tmp, { recursive: true, force: true })
}

// Pergunta — lstatSync sobre um symlink REAL (fs.symlinkSync tipo 'file').
// Em windows-latest sem Developer Mode/admin, a criacao costuma falhar com
// EPERM — sinal em si, impresso cru, sem contornar (mesmo padrao de
// probe.go/cmdLstatSymlink).
function cmdLstatSymlink() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-probe-symlink-'))
  printTempDirInfo(tmp)
  const target = path.join(tmp, 'target.txt')
  fs.writeFileSync(target, 'trackfw-probe-target\n')
  const link = path.join(tmp, 'link.txt')
  try {
    fs.symlinkSync(target, link, 'file')
  } catch (err) {
    process.stdout.write(
      `lstat-symlink create_err=${err.message} (esperado sem Developer Mode/admin — sinal em si, nao falha da sonda)\n`
    )
    fs.rmSync(tmp, { recursive: true, force: true })
    return
  }
  printLstat('lstat-symlink', link, lstatOrError(link))
  fs.rmSync(tmp, { recursive: true, force: true })
}

// Pergunta central desta extensao (AC3 da REQ) — lstatSync sobre uma
// JUNCTION criada por `mklink /J` (mesmo mecanismo que probe.go e probe.py
// usam — ver mklinkJunction acima). Não exige privilégio, ao contrário do
// symlink real acima. A pergunta bruta: libuv/Node marca isSymbolicLink()
// para esse reparse point, ou nao?
function cmdLstatJunction() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-probe-junction-'))
  printTempDirInfo(tmp)
  const targetDir = path.join(tmp, 'targetdir')
  fs.mkdirSync(targetDir)
  const junction = path.join(tmp, 'junctionlink')
  const mklink = mklinkJunction(junction, targetDir)
  process.stdout.write(
    `lstat-junction mklink_output=${JSON.stringify((mklink.stdout || '') + (mklink.stderr || ''))} mklink_status=${mklink.status} mklink_error=${mklink.error ? mklink.error.message : null}\n`
  )
  if (mklink.status !== 0 || mklink.error) {
    process.stdout.write(
      'lstat-junction create_failed — sonda nao pode medir lstatSync sobre a junction nesta execucao\n'
    )
    fs.rmSync(tmp, { recursive: true, force: true })
    return
  }
  printLstat('lstat-junction', junction, lstatOrError(junction))

  // Comparacao direta: statSync (segue o link) sobre o mesmo caminho — mesmo
  // formato comparativo que o braco Go usa (stat-junction).
  try {
    const followed = fs.statSync(junction)
    process.stdout.write(
      `stat-junction(segue o link) path=${junction} isDirectory=${followed.isDirectory()}\n`
    )
  } catch (err) {
    process.stdout.write(`stat-junction(segue o link) path=${junction} err=${err.message}\n`)
  }
  fs.rmSync(tmp, { recursive: true, force: true })
}

// rmdir-junction — Pergunta 10 do workflow (ML-1A). fs.rmdirSync sobre uma
// junction cujo alvo esta VAZIO. Discriminante citado pelo achado do ML-0A
// (hades-tf): npm/src/integrations/manager.js:420 cleanEmpty depende de
// fs.readdirSync(directory).length, sem testar isDirectory() — aqui medimos
// diretamente se rmdirSync tem sucesso sobre a propria junction e se o alvo
// sobrevive, em vez de inferir do comportamento do readdirSync.
//
// ML-1B (corretiva de barreira, achado do hefesto-tf): a condicao real de
// cleanEmpty (manager.js:420) e um curto-circuito de 3 termos —
//   !fs.existsSync(directory) || fs.lstatSync(directory).isSymbolicLink() || fs.readdirSync(directory).length
// — e o ML-1A so media o termo do meio (isSymbolicLink, via lstat-junction)
// e o rmdirSync final. Os termos ① existsSync e ③ readdirSync NUNCA foram
// medidos sobre a junction: e o readdirSync que decide se a producao CHEGA
// ao rmdirSync (se lancar, cleanEmpty lanca e nunca remove; se devolver o
// conteudo do alvo vazio, prossegue). Medidos aqui, na MESMA fixture, antes
// do rmdirSync — sem veredito: valor cru ou erro cru, nunca aborta o step.
function cmdRmdirJunction() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-probe-rmdir-'))
  printTempDirInfo(tmp)
  const targetDir = path.join(tmp, 'targetdir')
  fs.mkdirSync(targetDir)
  const junction = path.join(tmp, 'junctionlink')
  const mklink = mklinkJunction(junction, targetDir)
  if (mklink.status !== 0 || mklink.error) {
    process.stdout.write(
      `rmdir-junction create_err=${JSON.stringify((mklink.stdout || '') + (mklink.stderr || ''))} mklink_error=${mklink.error ? mklink.error.message : null}\n`
    )
    process.stdout.write(
      'rmdir-junction create_failed — sonda nao pode medir rmdirSync sobre a junction nesta execucao\n'
    )
    fs.rmSync(tmp, { recursive: true, force: true })
    return
  }

  // ① existsSync — o primeiro termo do curto-circuito de cleanEmpty. Segue o
  // link (nao e lstat): mede se, do ponto de vista de existsSync, a junction
  // "existe" como diretorio navegavel.
  let existsResult
  try {
    existsResult = fs.existsSync(junction)
    process.stdout.write(`rmdir-junction fs.existsSync(junction)=${existsResult}\n`)
  } catch (err) {
    process.stdout.write(`rmdir-junction fs.existsSync(junction)_err=${err.message}\n`)
  }

  // ③ readdirSync — o portao que decide se cleanEmpty CHEGA ao rmdirSync.
  // Lancar aqui e RESULTADO, nao falha de infraestrutura: captura, imprime
  // o erro cru e segue — nunca aborta o step por causa do valor.
  try {
    const entries = fs.readdirSync(junction)
    process.stdout.write(
      `rmdir-junction fs.readdirSync(junction)=${JSON.stringify(entries)} length=${entries.length}\n`
    )
  } catch (err) {
    process.stdout.write(`rmdir-junction fs.readdirSync(junction)_err=${err.message}\n`)
  }

  let removeErr = null
  try {
    fs.rmdirSync(junction)
  } catch (err) {
    removeErr = err.message
  }
  process.stdout.write(`rmdir-junction fs.rmdirSync(junction)_err=${removeErr}\n`)

  let junctionStillExists
  try {
    fs.lstatSync(junction)
    junctionStillExists = true
  } catch {
    junctionStillExists = false
  }
  process.stdout.write(`rmdir-junction junction_ainda_existe=${junctionStillExists}\n`)

  let targetStillExists
  try {
    fs.lstatSync(targetDir)
    targetStillExists = true
  } catch {
    targetStillExists = false
  }
  process.stdout.write(`rmdir-junction alvo_ainda_existe=${targetStillExists}\n`)

  fs.rmSync(tmp, { recursive: true, force: true })
}

function printTableRow(runtime, target, statOrErr) {
  if (statOrErr instanceof Error) {
    process.stdout.write(`TABELA runtime=${runtime} target=${target} err=${statOrErr.message}\n`)
    return
  }
  const info = statOrErr
  process.stdout.write(
    `TABELA runtime=${runtime} target=${target} isSymbolicLink=${info.isSymbolicLink()} isDirectory=${info.isDirectory()} isFile=${info.isFile()}\n`
  )
}

// table — Pergunta 11 do workflow. Recria arquivo comum, symlink e junction
// do zero (fixture propria, isolada das perguntas anteriores) e imprime uma
// linha TABELA por alvo, mesmo prefixo/formato que probe.go e probe.py
// usam, para comparacao lado a lado no mesmo step do workflow (AC5). Sem
// veredito (AC6): so os bits crus que este runtime usa para decidir "e
// link?".
function cmdTable() {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'trackfw-probe-table-'))
  printTempDirInfo(tmp)

  const common = path.join(tmp, 'common.txt')
  fs.writeFileSync(common, 'x')
  printTableRow('node', 'arquivo', lstatOrError(common))

  const target = path.join(tmp, 'target.txt')
  fs.writeFileSync(target, 'x')
  const link = path.join(tmp, 'link.txt')
  try {
    fs.symlinkSync(target, link, 'file')
    printTableRow('node', 'symlink', lstatOrError(link))
  } catch (err) {
    process.stdout.write(`TABELA runtime=node target=symlink err_create=${err.message}\n`)
  }

  const targetDir = path.join(tmp, 'targetdir')
  fs.mkdirSync(targetDir)
  const junction = path.join(tmp, 'junctionlink')
  const mklink = mklinkJunction(junction, targetDir)
  if (mklink.status === 0 && !mklink.error) {
    printTableRow('node', 'junction', lstatOrError(junction))
  } else {
    process.stdout.write(
      `TABELA runtime=node target=junction err_create=${JSON.stringify((mklink.stdout || '') + (mklink.stderr || ''))} mklink_error=${mklink.error ? mklink.error.message : null}\n`
    )
  }

  fs.rmSync(tmp, { recursive: true, force: true })
}

const COMMANDS = {
  'lstat-common': cmdLstatCommon,
  'lstat-symlink': cmdLstatSymlink,
  'lstat-junction': cmdLstatJunction,
  'rmdir-junction': cmdRmdirJunction,
  table: cmdTable,
}

function main() {
  const sub = process.argv[2]
  if (!sub || !COMMANDS[sub]) {
    process.stderr.write(`uso: probe.js <${Object.keys(COMMANDS).join('|')}>\n`)
    process.exit(2)
  }
  COMMANDS[sub]()
}

main()
