---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md"
squad: ""
---

# Roadmap: `check-referential-integrity.sh` diz `OK` e sai 0 sobre árvore vazia — quinto gate vácuo, e está dentro do `parity`

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md -->
REQ: docs/req/REQ-2026-09-03-check-referential-integrity-diz-ok-e-sai-zero-sobre-arvore-vazia-sem-guarda-de-vacuidade.md

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

### ML-1A — **AC1** — Guarda de vacuidade: varredura que enumere **zero** itens **falha** ou reporta
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Guarda de vacuidade: varredura que enumere **zero** itens **falha** ou reporta
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 **Falsificação nas duas direções:** árvore vazia → **reprova**; árvore íntegra →
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 **Falsificação nas duas direções:** árvore vazia → **reprova**; árvore íntegra →
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — O gate emite **contagem** do que verificou. Os demais gates ≤0,11 s têm guarda ou
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — O gate emite **contagem** do que verificou. Os demais gates ≤0,11 s têm guarda ou
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Cenário em scripts/check-gates-falsify.sh.** Esta é a correção da **classe**,
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Cenário em scripts/check-gates-falsify.sh.** Esta é a correção da **classe**,
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — O gate resolve req_dir/adr_dirs/roadmap_dir do trackfw.yaml em vez de
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — O gate resolve req_dir/adr_dirs/roadmap_dir do trackfw.yaml em vez de
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — make quality verde e trackfw validate exit 0.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality verde e trackfw validate exit 0.
- [ ] build passes
- [ ] tests green
