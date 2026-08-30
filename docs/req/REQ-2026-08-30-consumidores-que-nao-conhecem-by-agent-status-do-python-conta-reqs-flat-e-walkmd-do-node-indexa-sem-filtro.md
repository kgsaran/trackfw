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

- [ ] **AC1** — `status` do Python conta REQs corretamente em `by_agent`; saída equivalente a Go e
      Node para o mesmo projeto.
- [ ] **AC2** — `walkMd` do Node usa o resolvedor canônico, com o mesmo filtro de infraestrutura de
      Go e Python.
- [ ] **AC3** — **Varredura**: enumerar todos os consumidores restantes que resolvem caminho sem o
      resolvedor. Corrigir a classe. Se sobrar algum por decisão, **documentar** o motivo.
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
