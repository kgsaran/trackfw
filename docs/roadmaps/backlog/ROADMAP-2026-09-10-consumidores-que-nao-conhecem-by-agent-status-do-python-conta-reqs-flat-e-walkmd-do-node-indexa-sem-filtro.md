---
status: backlog
date: 2026-09-10
req: "docs/req/REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-status-do-python-conta-reqs-flat-e-walkmd-do-node-indexa-sem-filtro.md"
squad: ""
---

# Roadmap: Consumidores que não conhecem `by_agent` — `status` do Python conta REQs flat, e `walkMd` do Node indexa sem filtro

> Created: 2026-09-10 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-status-do-python-conta-reqs-flat-e-walkmd-do-node-indexa-sem-filtro.md -->
REQ: docs/req/REQ-2026-08-30-consumidores-que-nao-conhecem-by-agent-status-do-python-conta-reqs-flat-e-walkmd-do-node-indexa-sem-filtro.md

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

### ML-1A — **AC1** — 🔴 **REESCRITO em 2026-09-04, issue #268:** _count_reqs_by_status **usa
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 **REESCRITO em 2026-09-04, issue #268:** _count_reqs_by_status **usa
- [ ] build passes
- [ ] tests green

### ML-1B — **AC1-bis** — 🔴 **A saída se contradiz sozinha, e isso é o gate:** o mesmo comando imprime
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1-bis** — 🔴 **A saída se contradiz sozinha, e isso é o gate:** o mesmo comando imprime
- [ ] build passes
- [ ] tests green

### ML-1C — **AC2** — walkMd do Node usa o resolvedor canônico, com o mesmo filtro de infraestrutura de
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — walkMd do Node usa o resolvedor canônico, com o mesmo filtro de infraestrutura de
- [ ] build passes
- [ ] tests green

### ML-1D — **AC3** — **Varredura**: enumerar todos os consumidores restantes que resolvem caminho sem o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — **Varredura**: enumerar todos os consumidores restantes que resolvem caminho sem o
- [ ] build passes
- [ ] tests green

### ML-1E — **AC4** — Gate falsificável cobrindo AC1 e AC2 nos runtimes afetados.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Gate falsificável cobrindo AC1 e AC2 nos runtimes afetados.
- [ ] build passes
- [ ] tests green

### ML-1F — **AC5** — Não regride nada da REQ irmã: união, não-seguir-symlink, violação, aviso de oculto,
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Não regride nada da REQ irmã: união, não-seguir-symlink, violação, aviso de oculto,
- [ ] build passes
- [ ] tests green

### ML-1G — **AC6** — make quality exit 0 e CI verde.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — make quality exit 0 e CI verde.
- [ ] build passes
- [ ] tests green
