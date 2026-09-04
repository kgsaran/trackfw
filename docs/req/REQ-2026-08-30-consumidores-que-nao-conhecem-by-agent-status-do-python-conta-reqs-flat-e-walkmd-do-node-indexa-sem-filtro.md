---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: ""
---

# REQ: Consumidores que não conhecem `by_agent` — `status` do Python conta REQs flat, e `walkMd` do Node indexa sem filtro

> Date: 2026-08-30 | Status: Open

## Motivation

Dois resíduos da REQ do resolvedor canônico
(`REQ-2026-08-29-namespace-de-agente-nao-declarado-...`), reportados por `apolo-tf` e `hefesto-tf` e
deixados fora do escopo dela:

**1. `pypi/trackfw/commands/status.py::_count_reqs_by_status`** nunca soube de `by_agent`: conta REQs
por listagem **flat** da raiz de `req_dir`. Em projeto `by_agent` o Inventory do `status` mostra
**0 REQs**, divergindo de Go e Node. Pré-existente, confirmado com o binário antes do ML-1A.

**2. `npm/src/validator/traceid.js::walkMd`** recorre sem filtro e sem consciência de `agents:`.
Go e Python passaram a filtrar infraestrutura ao herdar o resolvedor canônico; o Node ficou para trás
— assimetria **alargada** por aquele diff.

Os dois têm a mesma raiz: **consumidores que resolvem caminho por conta própria em vez de usar o
resolvedor canônico**. É o resíduo do mesmo defeito que a REQ irmã atacou.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **REESCRITO em 2026-09-04, issue #268:** `_count_reqs_by_status` **usa
      `resolve_req_files`** — não "conta corretamente em `by_agent`".

      **A redação anterior estava errada e um remédio ruim a satisfaria.** O defeito **não é de
      `by_agent`**: o layout `req_dir/<estado>/*.md` é válido em **todos** os modos. Reproduzido pelo
      arquiteto num projeto **flat**, com `roadmap_namespacing` ausente do `trackfw.yaml`:

      ```
      docs/req/backlog/REQ-x.md   <- unico arquivo de REQ
      go     : REQs 1
      node   : REQs 1
      python : REQs 0      <-
      ```

      Ramificar no `namespacing` — o remédio natural para a redação antiga — deixaria **esta**
      reprodução em pé **e satisfaria o AC1 como estava escrito**. A forma nova é a que o **D4 do
      `ADR-2026-09-03`** já manda, e é **verificável por leitura**.

- [ ] **AC1-bis** — 🔴 **A saída se contradiz sozinha, e isso é o gate:** o mesmo comando imprime
      `REQs 0` e, abaixo, lista a REQ em `blocked by not-accepted ADRs`. **Não precisa de comparação
      entre runtimes** — asserção sobre **uma** execução, sem instalar os três CLIs. É o gate do AC4,
      e é mais forte que a divergência.

      O discriminante **não é o runtime, é o arquivo**: dentro do mesmo `status.py`, a linha 133
      (`_blocked_reqs`) usa `resolve_req_files` e a 101 trata `by_agent`. **O contador é a única
      enumeração do arquivo que ignora o layout** — e o `req list` e o `context` do próprio Python
      enxergam as 4.
- [ ] **AC2** — `walkMd` do Node usa o resolvedor canônico, com o mesmo filtro de infraestrutura de
      Go e Python.
- [ ] **AC3** — **Varredura**: enumerar todos os consumidores restantes que resolvem caminho sem o
      resolvedor. Corrigir a classe. Se sobrar algum por decisão, **documentar** o motivo.
      🔴 **Alvo concreto já enumerado (issue #268), e não é do Python:** o `sync` enumera **flat nos
      3 runtimes** e **ignora o `req_dir` configurado** — `internal/sync/sync.go:43`
      (`filepath.Glob("docs/req/*.md")`), `npm/src/commands/sync.js:237` (`const reqDir = 'docs/req'`)
      e `pypi/trackfw/commands/sync.py:197,207`. Já era resíduo declarado do ML-1A do resolvedor;
      agora tem linha e runtime.
- [ ] **AC4** — Gate falsificável cobrindo AC1 e AC2 nos runtimes afetados.
- [ ] **AC5** — Não regride nada da REQ irmã: união, não-seguir-symlink, violação, aviso de oculto,
      ordenação declarado-primeiro.
- [ ] **AC6** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** alterar o resolvedor canônico, já entregue e coberto por 66 cenários.
- **Não** unificar o formato de saída do `roadmap list` do Python — divergência de **formatação**
  registrada como `gap` no `cli-parity.md`.

## Linked ADR
<!-- none: aplica decisões do ADR-2026-08-29. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
