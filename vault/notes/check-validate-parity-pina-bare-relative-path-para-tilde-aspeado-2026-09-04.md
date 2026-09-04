---
title: check-validate-parity.sh pinava "bare relative path" para o caso "~/…" aspeado
date: 2026-09-04
tags: [validate, credential-guard, parity, adr-2026-09-04]
---

## O que aconteceu

Ao implementar o D4 da `ADR-2026-09-04-caminho-posix-ancorado-num-config-lido-por-cli-de-agente-e-
absoluto-independente-do-so-host` (ML-3A do roadmap Windows), troquei a mensagem de
`cwdDependentReason`/`_cwd_dependent_reason` para o caso `"~/…"` aspeado: de `"with a bare relative
path"` para `"with a quoted tilde path"`. Os 3 CLIs, os testes unitários dos 3 CLIs, `go build`,
`go vet` e as suítes completas (Go/Node/Python) passaram limpos. Só `make quality` — especificamente
`scripts/check-validate-parity.sh` — falhou:

```
credential_guard_hook_resolvable parity (claude-tilde-quoted/go): mensagem inesperada — esperava
'with a bare relative path' em todas: ['... with a quoted tilde path — ...']
```

## Por que os testes unitários não pegaram isso

`internal/validator/validator_credential_guard_test.go`, `npm/tests/validator.test.js` e
`pypi/tests/test_validator.py` cada um testa **seu próprio runtime isoladamente** — todos os três
tinham a mesma asserção desatualizada (`bare relative path` para `~/…` aspeado), então corrigir os 3
em paralelo não expõe divergência nenhuma: cada CLI concorda consigo mesmo.

`scripts/check-validate-parity.sh` é o único gate que roda os **3 binários reais** contra a **mesma
fixture JSON** e compara mensagem **byte-a-byte entre eles** (`CG_MARKER_BARE` aplicado a
`cg-claude-tilde-quoted-{go,node,py}.json`). É o único lugar onde a string exata é contrato de
paridade, não só contrato de teste unitário — e por isso é o único lugar que teria detectado se eu
tivesse esquecido de atualizar **um** dos 3 runtimes (a suíte unitária desse runtime teria passado
sozinha, porque eu teria atualizado a asserção junto com o código errado).

## Correção aplicada

`scripts/check-validate-parity.sh`: `CG_MARKER_BARE` → novo `CG_MARKER_TILDE_QUOTED = "with a
quoted tilde path"` aplicado só ao caso `claude-tilde-quoted`; comentários dos blocos de fixture
atualizados. Também acrescentei o fixture `cg-claude-windows-drive` (`C:\Users\kg\scripts\...` →
classe 1, silencioso) para o mesmo gate cobrir o caso de integração do ML-3A (letra de unidade
ancorada por união, D1 da ADR), não só o predicado isolado.

## Regra prática para quem tocar `cwdDependentReason`/`classifyHookAnchorage` de novo

Antes de declarar aceite: rodar `bash scripts/check-validate-parity.sh` isolado (não só as suítes
unitárias dos 3 CLIs) — é o único gate que falsifica divergência de MENSAGEM entre runtimes, e falha
rápido (segundos), sem precisar do `make quality` completo (~13 min) para descobrir isso. Ver também
[[reason-do-guard-diverge-por-escaping-de-aspas-entre-python-e-go-node-2026-08-22]] para outro caso
onde a mensagem exata quebrou paridade de forma não óbvia.
