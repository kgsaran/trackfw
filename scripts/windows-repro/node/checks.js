'use strict'

// checks.js — verificacoes Node da suite de reproducao de defeito (camada 2)
// para ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-
// sob-demanda, ML-1C.
//
// Unico subcomando por enquanto: gatequote (item 7 reclassificado). Replica
// o MESMO primitivo que npm/src/commands/barrier.js:561 usa em producao:
// spawnSync(command, { shell: true, ... }) — no Windows, isso resolve para
// cmd.exe. run.ps1 chama este script com o MESMO literal de comando usado
// pelo checks.go (Go, via `sh -c`) e pelo checks.py (Python, via
// subprocess.run(shell=True)) e compara os 3 stdouts brutos.
//
// Executado via `node scripts/windows-repro/node/checks.js gatequote`.

const { spawnSync } = require('child_process')

// Precisa ser EXATAMENTE o mesmo literal usado em go/checks.go
// (gateQuoteCommand) e python/checks.py (GATE_QUOTE_COMMAND).
const GATE_QUOTE_COMMAND =
  "echo start > /dev/null 2>&1 && echo 'trackfw-gate-verdict-A' || echo 'trackfw-gate-verdict-B'"

function cmdGateQuote() {
  const result = spawnSync(GATE_QUOTE_COMMAND, {
    shell: true,
    encoding: 'utf8',
    stdio: 'pipe',
  })
  const stdout = result.stdout || ''
  const stderr = result.stderr || ''
  process.stdout.write(`STDOUT_BEGIN\n${stdout}\nSTDOUT_END\n`)
  process.stdout.write(`exit=${result.status}\n`)
  if (stderr) process.stdout.write(`stderr_tail=${JSON.stringify(stderr.slice(-400))}\n`)
}

const COMMANDS = { gatequote: cmdGateQuote }

function main() {
  const sub = process.argv[2]
  if (!sub || !COMMANDS[sub]) {
    process.stderr.write(`uso: checks.js <${Object.keys(COMMANDS).join('|')}>\n`)
    process.exit(2)
  }
  COMMANDS[sub]()
}

main()
