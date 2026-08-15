---
status: backlog
date: 2026-08-15
req: "docs/req/REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md"
squad: ""
---

# Roadmap: trackfw ship gera corpo de PR minimo sem agregar historico de commits da branch

> Created: 2026-08-15 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md -->
REQ: docs/req/REQ-2026-08-15-trackfw-ship-gera-corpo-de-pr-minimo-sem-agregar-historico-de-commits-da-branch.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none

### ML-1A — buildPRBody (Go) e equivalentes (Node/Python) passam a agregar
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] buildPRBody (Go) e equivalentes (Node/Python) passam a agregar
- [ ] build passes
- [ ] tests green

### ML-1B — Título do PR: se houver só 1 commit não-merge na branch, mantém o comportamento
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Título do PR: se houver só 1 commit não-merge na branch, mantém o comportamento
- [ ] build passes
- [ ] tests green

### ML-1C — Resolução de branch base para o git log <base>..HEAD: usar
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Resolução de branch base para o git log <base>..HEAD: usar
- [ ] build passes
- [ ] tests green

### ML-1D — Comportamento idêntico nos 3 CLIs (mesmo corpo de PR gerado para o mesmo histórico
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Comportamento idêntico nos 3 CLIs (mesmo corpo de PR gerado para o mesmo histórico
- [ ] build passes
- [ ] tests green

### ML-1E — Não quebra o design forge-agnóstico do ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Não quebra o design forge-agnóstico do ADR-2026-07-26-trackfw-ship-agnostico-de-forge.md
- [ ] build passes
- [ ] tests green

### ML-1F — --dry-run continua funcionando e mostra o corpo/título que seria usado.
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] --dry-run continua funcionando e mostra o corpo/título que seria usado.
- [ ] build passes
- [ ] tests green

### ML-1G — make quality passa sem novas divergências de paridade.
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] make quality passa sem novas divergências de paridade.
- [ ] build passes
- [ ] tests green

### ML-1H — Teste de regressão cobrindo o caso real que motivou esta REQ: branch com múltiplos
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] Teste de regressão cobrindo o caso real que motivou esta REQ: branch com múltiplos
- [ ] build passes
- [ ] tests green
