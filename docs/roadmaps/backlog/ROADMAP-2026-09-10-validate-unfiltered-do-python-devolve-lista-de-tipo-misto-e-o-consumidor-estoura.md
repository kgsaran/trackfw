---
status: backlog
date: 2026-09-10
req: "docs/req/REQ-2026-09-05-validate-unfiltered-do-python-devolve-lista-de-tipo-misto-e-o-consumidor-estoura.md"
squad: ""
---

# Roadmap: validate_unfiltered do Python devolve lista de tipo misto e o consumidor estoura

> Created: 2026-09-10 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-05-validate-unfiltered-do-python-devolve-lista-de-tipo-misto-e-o-consumidor-estoura.md -->
REQ: docs/req/REQ-2026-09-05-validate-unfiltered-do-python-devolve-lista-de-tipo-misto-e-o-consumidor-estoura.md

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

### ML-1A — **AC1** — validate_unfiltered devolve **um único tipo**. O contrato fica escrito, não
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — validate_unfiltered devolve **um único tipo**. O contrato fica escrito, não
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **O else do _enrich_items deixa de ser fail-open silencioso.** Ele é o que
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **O else do _enrich_items deixa de ser fail-open silencioso.** Ele é o que
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — Falsificação nas duas direções: com o contrato violado de propósito, o gate acusa;
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — Falsificação nas duas direções: com o contrato violado de propósito, o gate acusa;
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Paridade verificada, não presumida.** Go e Node podem já estar corretos por
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Paridade verificada, não presumida.** Go e Node podem já estar corretos por
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Guarda contra a próxima instância:** um teste que exercite o contrato de retorno
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Guarda contra a próxima instância:** um teste que exercite o contrato de retorno
- [ ] build passes
- [ ] tests green
