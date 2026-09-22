---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-01-gate-de-shell-detecta-reversao-na-grafia-literal-mas-nao-a-semantica-endurecer-para-checagem-comportamental.md"
squad: ""
---

# Roadmap: Gate de shell detecta reversão na grafia literal, mas não a semântica — endurecer para checagem comportamental

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-01-gate-de-shell-detecta-reversao-na-grafia-literal-mas-nao-a-semantica-endurecer-para-checagem-comportamental.md -->
REQ: docs/req/REQ-2026-09-01-gate-de-shell-detecta-reversao-na-grafia-literal-mas-nao-a-semantica-endurecer-para-checagem-comportamental.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap
**Status:** ⬜ Pendente
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

### ML-1A — **AC1** — 🔴 **Checagem comportamental, não textual.** O gate observa **qual interpretador é de
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 **Checagem comportamental, não textual.** O gate observa **qual interpretador é de
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Falsificação com as duas evasões reproduzidas aqui**: assinatura viva comentada;
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Falsificação com as duas evasões reproduzidas aqui**: assinatura viva comentada;
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — **Controle:** a árvore correta continua passando, e código legítimo que **mencione**
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — **Controle:** a árvore correta continua passando, e código legítimo que **mencione**
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — As duas metades passam a ter **tratamento consistente** quanto a comentários, ou a
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — As duas metades passam a ter **tratamento consistente** quanto a comentários, ou a
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **A mesma pergunta aplicada aos outros gates de regex.** hefesto-tf já observou
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **A mesma pergunta aplicada aos outros gates de regex.** hefesto-tf já observou
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — make quality e **CI** verdes.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality e **CI** verdes.
- [ ] build passes
- [ ] tests green
