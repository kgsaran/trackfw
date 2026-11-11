---
status: Done
date: 2026-11-02
author: "Zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-11-08-contrato-de-marcadores-do-roadmap-bilingue-parser-aceita-ingles-e-portugues-antes-de-o-gerador-emitir-ingles.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md"
---

# REQ: Idioma do produto: inglês primeiro, com legenda de status aceitando inglês e português

> Date: 2026-11-02 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

Decisão do KG em 2026-11-02 (opção A): o trackfw passa a ser **inglês primeiro**. O README é em inglês,
então quem chega espera o produto em inglês, e hoje ele vem misturado.

Medido na Wave 0 da REQ-2026-11-02 de primeira experiência (`docs/seguranca/2026-11-02-wave0-primeira-experiencia.md`,
R1–R3): cerca de 190 literais com acento em uns 30 arquivos de produto. Os maiores são
`internal/generators/scaffold.go` (43), `internal/commands/help.go` (41), `internal/identity/preset.go` (9),
`internal/generators/roadmap.go` (9) e `internal/generators/update.go` (8). Eles caem em três grupos:

1. **Mensagens do CLI**, por exemplo `roadmap … já existe em …`. Só texto.
2. **Conteúdo gerado no consumidor**: o `CLAUDE.md` gerado pelo `init` e os comandos `/trackfw:*`.
3. **Contrato que o binário lê**: a legenda `⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado`
   e o `**Status:**` dos MLs, lidos pelo `barrier` e pelo `validate`.

As mensagens do guard já são tratadas pela REQ de primeira experiência e ficam fora daqui.

## Acceptance Criteria
- [x] **AC1 — Inventário por grupo.** Uma tabela com cada sítio e o grupo dele (1, 2 ou 3), feita por grep de
  mecanismo, com a decisão de cada um.
- [x] **AC2 — Mensagens do CLI em inglês** (grupo 1), com teste que reprova o retorno do português nas
  mensagens emitidas.
- [x] **AC3 — Conteúdo gerado em inglês** (grupo 2). Consumidores existentes recebem a versão nova pelo
  `trackfw update`, sem perder edição local; o contrato atual do `update` decide o que é sobrescrito.
- [x] **AC4 — Marcadores de contrato bilíngues** (grupo 3): `barrier` e `validate` aceitam em inglês e em
  português todo marcador que leem do roadmap e da REQ, inclusive `**Wave gates:**` / `**Gates da wave:**`,
  `Lapsed:` / `Caducou:` e as palavras de término (`abandoned`, `cancelled`/`canceled`). O conjunto fechado
  de grafias está na ADR-2026-11-08. O acervo existente, deste repositório e de consumidores, continua
  passando sem edição: a saída de `barrier` e `validate` sobre `docs/roadmaps/` é idêntica entre o binário
  da `main` e o da branch. Os roadmaps novos nascem em inglês, numa tag **posterior** à que publica o leitor
  bilíngue (ADR D2).

  *Por que o escopo cresceu (2026-11-08):* a REQ original citava só a legenda e o `**Status:**`. A leitura
  dos parsers antes da ADR mostrou que `**Gates da wave:**`, `Caducou:` e as palavras de término também só
  existem em português, e que uma wave sem cabeçalho de gates reconhecido conta zero gates e passa. É a
  mesma causa (contrato lido pelo binário fixado em português), então entra na mesma REQ.
- [x] **AC5 — Presets de identidade** com nomes próprios em português (por exemplo `chaves` e `turma`):
  decisão escrita sobre manter ou traduzir.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-08-contrato-de-marcadores-do-roadmap-bilingue-parser-aceita-ingles-e-portugues-antes-de-o-gerador-emitir-ingles.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md

## Negative scope

- Não cria seleção de idioma (`--lang`) nem i18n.
- Não reescreve a documentação de `docs/` (ADRs, REQs, pareceres e notas continuam em português).
- Não trata as mensagens do guard, que ficam na REQ de primeira experiência.
