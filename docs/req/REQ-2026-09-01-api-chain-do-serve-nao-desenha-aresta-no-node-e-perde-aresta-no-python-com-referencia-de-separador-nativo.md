---
status: Superseded
date: 2026-09-01
author: "zeus-tf"
adr: ""
roadmap: ""
---

# REQ: `/api/chain` do `serve` não desenha aresta no Node e perde aresta no Python com separador nativo

> Date: 2026-09-01 | Status: Superseded

## Motivation

Achado do `hefesto-tf` na barreira final da `REQ-2026-08-30-caminho-portavel-...` (PR #231),
**verificado ao vivo nos dois runtimes**, não inferido:

| runtime | comportamento |
|---|---|
| Go | corrigido no PR #231 — normaliza id do nó e `edge.To` |
| Python (`pypi/trackfw/serve/api_chain.py`) | com `/` desenha a aresta; com `\\` produz **zero** |
| Node (`npm/src/serve/api_chain.js`) | **não desenha aresta nenhuma — nem com referência limpa** |

**Nenhum dos dois é regressão do PR #231.** O Python tem o mesmo defeito de separador que o Go tinha;
o Node tem um **bug estrutural mais amplo e anterior**: o grafo do board simplesmente não liga nada.

## Por que importa mais do que "um board feio"

O `/api/chain` é a **visualização da cadeia de governança** — ADR → REQ → ROADMAP. Uma aresta que
some não produz erro: **o grafo desenha, parece correto, e a ligação não está lá.**

É a mesma classe que nos custou sete ocorrências na sessão de 2026-08-30/09-01: **o mecanismo dá
sinal de sucesso enquanto o controle está inerte.** Aqui é pior que um gate desligado, porque há um
artefato visual afirmando uma cadeia que ele não conseguiu montar.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Diagnosticar o Node antes de corrigir.** Ele não desenha aresta **nem com
      referência limpa** — então o defeito **não é** de separador, é outro. Corrigir separador ali
      sem entender a causa raiz produziria correção que não corrige nada, e um teste verde sobre um
      grafo ainda vazio.
- [ ] **AC2** — Python normaliza separador na montagem do id do nó e do `to` da aresta, como o Go
      passou a fazer no PR #231.
- [ ] **AC3** — Falsificação nas duas direções, **por runtime**: referência com `\\` desenha a
      aresta; referência com `/` **continua** desenhando. E o controle que falta hoje: **um grafo com
      N ligações declaradas produz N arestas**, não zero.
- [ ] **AC4** — Gate ou teste que reprove um grafo com **zero arestas** quando há ligações
      declaradas. É a asserção que teria pego o defeito do Node — nenhum teste atual afirma que o
      grafo **liga** alguma coisa.
- [ ] **AC5** — Paridade nos 3 runtimes, verificada por comparação de saída.

## Negative Scope

- **Não** reabrir a `REQ-2026-08-30`; o Go já foi corrigido e medido.
- **Não** presumir que o defeito do Node é de separador — a AC1 existe exatamente por isso.

## Linked ADR

ADR: <!-- avaliar após o diagnóstico do Node. -->

## Linked Roadmap

Roadmap:

## Fechamento — triagem de 2026-09-18

**Veredito:** Superseded. **Motivo medido:** bracos Node e Python removidos pela v8; o serve Go responde 505 nodes / 531 edges

Fechada na triagem por evidência das REQs abertas (`docs/triagem-reqs-abertas-2026-09-18.md`),
executada a pedido do KG. Esta triagem é o **AC12 da `REQ-2026-09-12-v8-um-binario-muitos-canais`**
e os **AC10-AC12** da REQ irmã, que pediam classificar as REQs abertas e fechar aquelas cuja causa a
v8 removeu — e que nunca haviam sido executados.
