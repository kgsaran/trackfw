---
status: backlog
date: 2026-09-26
req: "docs/req/REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md"
squad: ""
---

# Roadmap: o harness do arquiteto nao proibe sobrepor o modelo do subagente e o roteamento de custo e ignorado em silencio

> Created: 2026-09-26 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md -->
REQ: docs/req/REQ-2026-09-26-o-harness-do-arquiteto-nao-proibe-sobrepor-o-modelo-do-subagente-e-o-roteamento-de-custo-e-ignorado-em-silencio.md

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

### ML-1A — **O Dispatch contract proíbe explicitamente passar model**, com a razão escrita (roteamento de
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **O Dispatch contract proíbe explicitamente passar model**, com a razão escrita (roteamento de
- [ ] build passes
- [ ] tests green

### ML-1B — O contrato manda ler **name: e model:** do arquivo do agente, e diz que o segundo é **do
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] O contrato manda ler **name: e model:** do arquivo do agente, e diz que o segundo é **do
- [ ] build passes
- [ ] tests green

### ML-1C — 🔴 **Falsificação nas duas direções:** um architect.md gerado **sem** a proibição reprova; o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] 🔴 **Falsificação nas duas direções:** um architect.md gerado **sem** a proibição reprova; o
- [ ] build passes
- [ ] tests green

### ML-1D — ⚠️ **Enumerar os outros sítios da mesma classe antes de fechar** (Regra Dura): o Dispatch
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] ⚠️ **Enumerar os outros sítios da mesma classe antes de fechar** (Regra Dura): o Dispatch
- [ ] build passes
- [ ] tests green

### ML-1E — O golden internal/integrations/testdata/architect.subagent.golden.md acompanha, e a mudança é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] O golden internal/integrations/testdata/architect.subagent.golden.md acompanha, e a mudança é
- [ ] build passes
- [ ] tests green

### ML-1F — make quality e **CI** verdes
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] make quality e **CI** verdes
- [ ] build passes
- [ ] tests green
