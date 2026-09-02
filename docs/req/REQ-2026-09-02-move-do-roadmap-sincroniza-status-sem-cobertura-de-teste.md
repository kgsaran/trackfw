---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-cobrir-a-sincronia-de-status-do-move-com-falsificacao.md"
---

# REQ: `roadmap move` sincroniza `status:` e a linha humana, e nada testa isso

> Date: 2026-09-02 | Status: Open

## Motivation

**Isto não é correção de defeito.** O comportamento em `internal/generators/roadmap.go` está certo:
`MoveRoadmap` chama `rewriteRoadmapStatus`, que reescreve o `status:` do frontmatter **e** o trecho
`| Status: <valor>` da linha de cabeçalho do corpo, para que o arquivo não vá parar em `done/`
ainda declarando `status: wip` — a incoerência de que a regra `folder_status` do validator reclama.

O que falta é **prova por teste**. `rewriteRoadmapStatus` tem sete decisões de escopo explícitas nos
comentários dela — só a primeira `status:` do frontmatter, só o primeiro `| Status: ` antes de `##`,
nunca inventar a chave, nunca inventar a linha — e nenhuma delas está coberta. Cada uma é uma
maneira de uma refatoração futura corromper documento do usuário em silêncio.

O caso mais feio é o negativo: um roadmap **sem frontmatter** cujo corpo tenha uma linha começando
com `status:`. Uma substituição global — a simplificação óbvia que alguém faria — reescreveria essa
linha do corpo. Nada hoje pegaria.

## Acceptance Criteria

- [ ] **AC1** — Cobrir as duas direções da sincronia: frontmatter e linha humana, incluindo o
      formato herdado com emoji.
- [ ] **AC2** — Cobrir os três negativos: sem frontmatter sai byte a byte idêntico; frontmatter sem
      `status:` não ganha o campo; `status:` no corpo não é tocado.
- [ ] **AC3** — 🔴 **Falsificação.** Desligar `rewriteRoadmapStatus` em `MoveRoadmap` deve acender os
      testes. Um teste que passa e nunca acende não prova nada.
- [ ] **AC4** — Os testes rodam em `flat`, sem depender do layout deste ou daquele repositório.

## Negative Scope

**Não muda comportamento.** Nenhuma linha de `roadmap.go` é tocada. Se algum destes testes reprovar
em `main`, é achado — não é ajuste a fazer no teste.

**Não cobre `analyzing`**, que já tem `TestMoveRoadmap_AnalyzingFlat` e `_AnalyzingByAgent`.

## Linked ADR
<!-- Cobertura de comportamento existente; sem decisão nova a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-cobrir-a-sincronia-de-status-do-move-com-falsificacao.md
