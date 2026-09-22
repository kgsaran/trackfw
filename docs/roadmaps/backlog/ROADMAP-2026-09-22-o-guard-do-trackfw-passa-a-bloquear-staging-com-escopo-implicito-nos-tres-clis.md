---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md"
squad: ""
---

# Roadmap: o guard do trackfw passa a bloquear staging com escopo implicito nos tres CLIs

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md -->
REQ: docs/req/REQ-2026-09-05-o-guard-do-trackfw-passa-a-bloquear-staging-com-escopo-implicito-nos-tres-clis.md

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

### ML-1A — **AC1** — O guard bloqueia git add de escopo implícito: -A, --all, ., -u,
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — O guard bloqueia git add de escopo implícito: -A, --all, ., -u,
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — Staging por caminho explícito **continua passando**, inclusive múltiplos caminhos e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — Staging por caminho explícito **continua passando**, inclusive múltiplos caminhos e
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Controle de não-afrouxamento**, e é o critério principal: enumerar o que passou a
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Controle de não-afrouxamento**, e é o critério principal: enumerar o que passou a
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **falham**.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Guarda de vacuidade:** com a regra desligada, os cenários de bloqueio **falham**.
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Mensagem de commit contendo o texto git add -A **não** é tratada como comando —
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Mensagem de commit contendo o texto git add -A **não** é tratada como comando —
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **A mensagem ensina.** Mostra git status --short para enumerar e a forma válida.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **A mensagem ensina.** Mostra git status --short para enumerar e a forma válida.
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — Os limites de tokenização ficam **declarados** (git${IFS}add, {git,add},
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — Os limites de tokenização ficam **declarados** (git${IFS}add, {git,add},
- [ ] build passes
- [ ] tests green

### ML-1H — **AC8** — 🔴 **O gate de paridade cobre a regra nova.** Os guards existentes têm cobertura
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC8** — 🔴 **O gate de paridade cobre a regra nova.** Os guards existentes têm cobertura
- [ ] build passes
- [ ] tests green
