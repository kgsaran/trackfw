---
status: Open
date: 2026-11-23
author: "zeus-tf (pedido do KG em 2026-11-22 e 2026-11-23)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md"
---

# REQ: Site do trackfw com visão do produto: fluxo desenhado, cenários de uso e imagens geradas pelo binário

> Date: 2026-11-23 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
O site (`site/`, https://kgsaran.github.io/trackfw/) lista comandos, mas não mostra para que o trackfw serve. A direção do
produto desde 2026-10-28 é consolidar e aumentar a adoção. O KG pediu um site mais elaborado, com imagens,
possibilidades de uso e o desenho do fluxo de trabalho do trackfw.

**Decisões do KG:**
- 2026-11-22: método protótipo primeiro (§9 das regras globais); imagens geradas a partir do binário.
- 2026-11-23: GoatCounter pulado; protótipo em inglês primeiro, pt-BR na implementação final.

Cenários de uso iniciais, a validar no conceito: dev solo com agente de IA; time com CI; empresa que só libera npm/pip.

## Acceptance Criteria
- [ ] AC1: conceito (estrutura da home, cenários de uso, desenho do fluxo) aprovado pelo KG antes de qualquer código de
  produção
- [ ] AC2: protótipo navegável e isolado, validado visualmente pelo KG em rodadas curtas; descartado antes do PR final
- [ ] AC3: ADR com o modelo aprovado e roadmap de implementação escritos depois da validação
- [ ] AC4: site final em inglês e pt-BR, com imagens geradas a partir do binário por script versionado, sem dado
  pessoal nem caminho local nas imagens
- [ ] AC5: o teste do Quick start (README e guia do site) continua verde; build do site rc=0; capturas headless das
  páginas novas
- [ ] AC6: `make quality` rc=0 e CI verde

## Negative scope
- Não instala analytics (GoatCounter pulado pelo KG).
- Não muda o comportamento do CLI.
- Não muda o domínio nem a hospedagem (GitHub Pages, `base: /trackfw/`).
- Não cria job novo de CI (gates novos congelados).

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-23-site-do-trackfw-com-visao-do-produto-fluxo-desenhado-cenarios-de-uso-e-imagens-geradas-pelo-binario.md
