'use strict'

// checks.js — verificacoes Node da suite de reproducao de defeito (camada 2)
// para ROADMAP-2026-08-30-job-de-windows-largo-que-nasce-vermelho-e-sonda-
// sob-demanda, ML-1C.
//
// ROADMAP-2026-09-05-retarget-dos-checks-de-camada-2 (ML-2D): o unico
// subcomando deste arquivo ("gatequote") foi REMOVIDO. Ele replicava
// npm/src/commands/barrier.js fora do `barrier` real — exatamente o padrao
// que este roadmap corrige. O item 7 do run.ps1 agora invoca `trackfw
// barrier` de verdade (via npm/bin/trackfw), nao mais este arquivo.
//
// Mantido presente (em vez de apagado) para nao quebrar nenhuma referencia
// externa que ainda exista a `scripts/windows-repro/node/checks.js` e para
// simetria com go/checks.go e python/checks.py, que continuam com
// subcomandos ativos. Sem consumidor conhecido hoje — confirmado por grep
// em .ps1/.yml/.md antes desta ML.

function main() {
  const sub = process.argv[2]
  process.stderr.write(`checks.js: nenhum subcomando ativo (era so "gatequote", removido pelo ML-2D)${sub ? `; recebido "${sub}"` : ''}\n`)
  process.exit(2)
}

main()
