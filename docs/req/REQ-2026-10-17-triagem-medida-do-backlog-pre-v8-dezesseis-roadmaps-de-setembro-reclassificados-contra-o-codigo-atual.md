---
status: Done
date: 2026-10-17
author: "zeus-tf"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-17-triagem-medida-do-backlog-pre-v8-dezesseis-roadmaps-de-setembro-reclassificados-contra-o-codigo-atual.md"
---

# REQ: triagem medida do backlog pre-v8 — dezesseis roadmaps de setembro reclassificados contra o codigo atual

> Date: 2026-10-17 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

O backlog tem 16 roadmaps (15 em `backlog/`, 1 em `blocked/`), todos de 2026-09-08 a 2026-09-26 —
**anteriores à v8**, que trocou três implementações por uma só em Go. A REQ-2026-09-03 tinha o mesmo
perfil e, reclassificada **por medição**, mostrou parte dos itens sumida com a v8 e parte mudada de
forma. Sem a mesma triagem, o tamanho real do backlog é desconhecido e a priorização é chute.

## Acceptance Criteria
- [x] **AC1** — Cada um dos 16 roadmaps tem veredito **sumiu com a v8** / **vivo** / **mudou de forma**
      / **já corrigido por outro PR**, com **evidência no código atual** (arquivo:linha, comando e saída,
      ou PR que o fechou) — nunca só pelo título.
- [x] **AC2** — Nenhum item é abandonado sem evidência de que o defeito **não reproduz** hoje; na dúvida,
      fica vivo. O ônus é de quem fecha.
- [x] **AC3** — Itens vivos com **mesma causa** são agrupados (Regra Dura de Causa Raiz) e recebem
      prioridade; segurança primeiro.
- [x] **AC4** — As REQs abertas ligadas a esses roadmaps são reconciliadas (Open / Abandoned / Done)
      junto com o roadmap.
- [x] **AC5** — Nenhuma linha de código de produto nesta REQ: é triagem. Correções viram MLs/REQs
      depois, com o veredito como insumo.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-17-triagem-medida-do-backlog-pre-v8-dezesseis-roadmaps-de-setembro-reclassificados-contra-o-codigo-atual.md
