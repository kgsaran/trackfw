---
status: wip
date: 2026-09-03
req: "docs/req/REQ-2026-09-03-status-do-go-e-do-node-perde-em-silencio-o-que-nao-reconhece.md"
---

# ROADMAP: bucket Other no status de Go e Node

> Date: 2026-09-03 | Status: wip

REQ: docs/req/REQ-2026-09-03-status-do-go-e-do-node-perde-em-silencio-o-que-nao-reconhece.md
ADR:

## ML-1A — Espelhar o Python nos outros dois

**Status:** ✅ Concluído

`internal/validator/validator.go`: `default: reqOther++` no `switch`, e a linha de saída passa a
montar `reqDetail` com o sufixo condicional.

`npm/src/validator/index.js`: `else reqOther++`, mesma montagem condicional.

Nenhuma linha do Python foi tocada — ele já estava certo, e é a referência.

## ML-1B — Falsificação nas duas direções

**Status:** ✅ Concluído

**(a) Com as 8 grafias** (`open`, `done`, `closed`, `abandoned`, `backlog`, `wip`, `approved`,
`lixo-absoluto`), os três concordam:

```
go     : REQs  8  (1 Open · 1 Done · 1 Closed · 5 Other)
node   : REQs  8  (1 Open · 1 Done · 1 Closed · 5 Other)
python : REQs  8  (1 Open · 1 Done · 1 Closed · 5 Other)
```

**(b) Controle — só o vocabulário canônico.** O `Other` some nos três:

```
go     : REQs  3  (1 Open · 1 Done · 1 Closed)
node   : REQs  3  (1 Open · 1 Done · 1 Closed)
python : REQs  3  (1 Open · 1 Done · 1 Closed)
```

Esse controle é o que prova a compatibilidade: projeto com vocabulário limpo vê exatamente a linha
de hoje.

## ML-1C — Não-regressão dos testes existentes

**Status:** ✅ Concluído

`TestGetStatus_InventoryREQsDiscriminadas`, que pina a string `(1 Open · 1 Done · 1 Closed)`,
**passa**.

Duas falhas aparecem na suíte, e as duas são **idênticas no `upstream/main` limpo**, medidas em
worktree destacado antes de atribuir:

```
--- FAIL: TestFolderStatus_DiretorioNaoLegivel_P2   (Windows nao produz diretorio ilegivel assim)
FAILED pypi/tests/test_commands_basic.py::TestRealCommands::test_status_uses_real_handler
```

O Node dá `pass 0 / fail 1` nos dois lados — é a carga do diretório de testes, não um teste.
