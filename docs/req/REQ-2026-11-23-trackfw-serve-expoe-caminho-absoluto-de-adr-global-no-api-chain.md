---
status: Open
date: 2026-11-23
author: "zeus-tf (aberta a pedido do KG em 2026-11-23)"
adr: ""
roadmap: "docs/roadmaps/backlog/ROADMAP-2026-11-23-trackfw-serve-expoe-caminho-absoluto-de-adr-global-no-api-chain.md"
---

# REQ: trackfw serve expõe caminho absoluto de ADR global no /api/chain

> Date: 2026-11-23 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
Achado do threat model da REQ-2026-11-23 do site novo (`docs/seguranca/2026-11-23-wave0-site-novo-threat-model.md`, T2).
Medido com canário e HOME isolado: com `adr_dirs` contendo uma entrada `~/...` (ADR global), o endpoint `/api/chain` do
`trackfw serve` devolve no campo `id` dos nós o caminho absoluto expandido (por exemplo,
`/Users/<usuário>/global-adrs/ADR-x.md`). Uma captura do painel de cadeia, numa issue, num post ou na documentação,
publica o nome de usuário e o home. O `/api/board` não tem o problema: usa só o basename.

O `serve` também carrega bibliotecas de CDN sem SRI (outra causa, citada no mesmo threat model). A varredura do AC3
decide se isso entra aqui ou vira REQ própria, com a medição escrita.

## Acceptance Criteria
- [ ] Nenhuma resposta de `/api/*` do `trackfw serve` contém caminho absoluto do home; ADR global aparece como `~/...`
  ou pelo basename, decisão registrada em ADR (braço e contra-braço medidos com canário)
- [ ] O painel de cadeia continua resolvendo os vínculos entre ADR, REQ e roadmap
- [ ] Varredura dos demais endpoints e saídas do `serve` pelo mesmo mecanismo (caminho expandido exposto)
- [ ] `make quality` rc=0 e CI verde

## Negative scope
- Não muda o leitor de `adr_dirs` nem a expansão de `~` usada para ler os arquivos.
- Não muda o site estático (REQ-2026-11-23 do site novo).

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/backlog/ROADMAP-2026-11-23-trackfw-serve-expoe-caminho-absoluto-de-adr-global-no-api-chain.md
