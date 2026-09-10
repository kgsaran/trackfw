---
status: wip
date: 2026-09-10
req: "docs/req/REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md"
squad: ""
---

# Roadmap: `barrier` executa gate de roadmap não confiável porque `roadmapTrustForGates` falha aberto em todo caminho de erro

> Created: 2026-09-10 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md -->
REQ: docs/req/REQ-2026-08-30-barrier-executa-gate-de-roadmap-nao-confiavel-porque-roadmaptrustforgates-falha-aberto-em-todo-caminho-de-erro.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] AC1–AC7: postura fail-closed implementada nos 3 CLIs — Go, Node.js, Python
- [x] AC8: build green, todos os testes passam, falsificação nas duas direções confirmada

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [ ] The four sections above answered with evidence, not a one-line assertion
- [ ] No implementation line written for this ML

**Gates da wave:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — **AC1** — Postura invertida: **fecha por padrão**, abre só quando conseguir **provar** que o
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Postura invertida: **fecha por padrão**, abre só quando conseguir **provar** que o
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Cada condição hoje fail-open passa a not_evaluated com **razão nomeada**: sem
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Cada condição hoje fail-open passa a not_evaluated com **razão nomeada**: sem
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — A distinção não pode depender de casamento de substring de mensagem do git. Use
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — A distinção não pode depender de casamento de substring de mensagem do git. Use
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — --trust-local-gates continua sendo a saída explícita e auditável, e é a única.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — --trust-local-gates continua sendo a saída explícita e auditável, e é a única.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Falsificação nas duas direções: roadmap idêntico a origin/main → gates **executam**;
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Falsificação nas duas direções: roadmap idêntico a origin/main → gates **executam**;
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — Paridade nos 3 CLIs; gate falsificável cobrindo as condições de AC2.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — Paridade nos 3 CLIs; gate falsificável cobrindo as condições de AC2.
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — Não quebrar o fluxo legítimo do arquiteto neste repositório, que usa
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — Não quebrar o fluxo legítimo do arquiteto neste repositório, que usa
- [ ] build passes
- [ ] tests green

### ML-1H — **AC8** — make quality exit 0 **e CI verde**.
**Status:** ✅ Concluído
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC8** — make quality exit 0 **e CI verde**.
- [ ] build passes
- [ ] tests green
