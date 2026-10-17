---
status: done
date: 2026-10-17
req: "docs/req/REQ-2026-10-17-triagem-medida-do-backlog-pre-v8-dezesseis-roadmaps-de-setembro-reclassificados-contra-o-codigo-atual.md"
squad: "hefesto-tf"
---

# Roadmap: triagem medida do backlog pre-v8 — dezesseis roadmaps de setembro reclassificados contra o codigo atual

> Created: 2026-10-17 | Status: done

## Context
REQ: docs/req/REQ-2026-10-17-triagem-medida-do-backlog-pre-v8-dezesseis-roadmaps-de-setembro-reclassificados-contra-o-codigo-atual.md

16 roadmaps de setembro, anteriores à v8. Triagem por **medição no código atual** (`main` em
`f8870c79`, v9.7.0), sem código de produto.

## Acceptance Criteria
- [x] Veredito com evidência para os 16 (AC1/AC2 da REQ)
- [x] Agrupamento por causa e prioridade dos vivos (AC3)
- [x] REQs ligadas reconciliadas e roadmaps movidos pelo `trackfw roadmap move` (AC4)

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
### ML-0A — Threat model
**Status:** ✅ Concluído (zeus-tf, 2026-10-17)
1. **Superfície:** só artefatos de governança (roadmaps, REQs, um doc de triagem). Lista fechada: os
   16 de `docs/roadmaps/backlog/` e `blocked/` em 2026-10-17.
2. **Ameaça:** abandonar defeito vivo com aparência de triado — o defeito sai da fila e continua em
   produção. Mitigação: AC2 (evidência de não-reprodução; na dúvida, vivo) e auditoria do arquiteto
   por amostra contra o código.
3. **Falsificação nas duas direções:** "sumiu" exige mostrar o sítio inexistente ou o comportamento
   correto hoje; "vivo" exige mostrar o sítio defeituoso hoje.
4. **Resíduo:** itens cujo teste exige Windows ficam "vivo — não medido" se a VM/CI não forem usados.

## Wave 1 — Triagem (3 MLs em paralelo, só leitura)
### ML-1A — segurança e guard (5 roadmaps) · `hefesto-tf`
**Status:** ✅ Concluído
### ML-1B — gates vácuos e testes (6 roadmaps) · `hefesto-tf`
**Status:** ✅ Concluído
### ML-1C — Windows, mensagens, higiene e processo (5 roadmaps) · `hefesto-tf`
**Status:** ✅ Concluído
**Critério:** tabela por roadmap: veredito, evidência (arquivo:linha / comando+saída / PR), causa
agrupável, REQ ligada e recomendação.

## Wave 2 — Consolidação · `zeus-tf`
### ML-2A — Consolidação e movimentos
**Status:** ✅ Concluído — `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`
Doc `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`; movimentos de roadmap e REQ; prioridade.

## Resultado (2026-10-17)

16 roadmaps → **10** seguem em `backlog/` (com escopo reescrito por nota de triagem em cada um); 4 → `abandoned/`
(um deles absorvido por mesma causa), 2 → `done/`. Prioridade e evidência no doc da triagem.
