---
status: Open
date: 2026-09-03
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-03-bucket-other-no-status-de-go-e-node.md"
---

# REQ: o `status` do Go e do Node perde em silêncio o que não reconhece

> Date: 2026-09-03 | Status: Open

## Motivation

`_count_reqs_by_status` do Python agrupa em **quatro** buckets — `open`, `done`, `closed` e
`other` — e imprime o quarto quando ele não é zero:

```python
# pypi/trackfw/commands/status.py:58
counts = {"total": len(files), "open": 0, "done": 0, "closed": 0, "other": 0}
# :199
req_detail += f" · {req_counts['other']} Other"
```

Go e Node param nos três primeiros. O `switch`/`if` do contador não tem `default`/`else`, então
toda grafia fora do vocabulário canônico **é lida, contada no total, e descartada da quebra**.

Medido com 8 REQs, uma por grafia (`open`, `done`, `closed`, `abandoned`, `backlog`, `wip`,
`approved`, `lixo-absoluto`):

```
go     : REQs  8  (1 Open · 1 Done · 1 Closed)              ← 5 somem
node   : REQs  8  (1 Open · 1 Done · 1 Closed)              ← 5 somem
python : REQs  8  (1 Open · 1 Done · 1 Closed · 5 Other)
```

O total bate nos três. **A quebra é que mente por omissão** — quem lê não tem como saber que 5
REQs existem e não estão em lugar nenhum da conta.

## Por que isso importa, com número

No fork que eu mantenho, isso escondeu **24 de 53 REQs**. Descobri porque fui somar
`8 Open + 21 Done + 0 Closed` e não deu 53. Sete grafias em uso — `done`, `approved`, `Open`,
`backlog`, `abandoned`, `Proposed`, e uma sem `status` nenhum.

Enquanto isso, `req move` aceita qualquer string nos três runtimes (`req move <nome> lixo-absoluto`
grava `status: lixo-absoluto` sem reclamar). As duas pontas combinadas produzem acervo invisível
sem nenhum sinal.

## Acceptance Criteria

- [ ] **AC1** — Go e Node ganham o bucket `Other`, com a mesma semântica do Python: tudo que não é
      `open`/`done`/`closed` cai nele.
- [ ] **AC2** — O sufixo `· N Other` só aparece quando `N > 0`, como no Python. Projeto com
      vocabulário limpo continua vendo exatamente a linha de hoje.
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** Com as 8 grafias, os 3 runtimes dizem
      `5 Other`; com só as 3 canônicas, nenhum imprime `Other`.
- [ ] **AC4** — O teste que pina `(1 Open · 1 Done · 1 Closed)` continua passando.

## Negative Scope

**Não muda o `req move`.** Fazer o comando recusar valores quebraria todo adotante com `approved`
ou `backlog` no acervo — o meu inclusive — e a escolha do vocabulário canônico é de quem mantém.

**Não decide o que fazer com `abandoned`.** Hoje ele cai no `Other`, o que ao menos o torna
visível. Se merece bucket próprio — porque "desistimos" não é "resolvemos" — é decisão separada.

**Não toca no total.** Ele já estava certo nos três.

## Linked ADR
<!-- Correção de paridade sobre comportamento já existente no Python; sem decisão nova. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-03-bucket-other-no-status-de-go-e-node.md
