---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md"
squad: ""
---

# Roadmap: `init` e `discover` geram dois workflows que rodam a mesma validação, com instaladores diferentes

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md -->
REQ: docs/req/REQ-2026-09-02-init-e-discover-geram-dois-workflows-que-rodam-a-mesma-validacao-com-instaladores-diferentes.md

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

### ML-1A — **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 **Determinar por que existem dois**, com evidência (histórico, ADR, comportamento
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Se não houver: um único workflow gerado, com o instalador escolhido e **justificado**.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Controle:** o caminho de adoção que hoje depende do workflow removido **continua
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — Paridade nos 3 CLIs — a duplicação existe nos três.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — Paridade nos 3 CLIs — a duplicação existe nos três.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Migração para quem **já tem os dois** instalados: o update remove o obsoleto, ou o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Migração para quem **já tem os dois** instalados: o update remove o obsoleto, ou o
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
