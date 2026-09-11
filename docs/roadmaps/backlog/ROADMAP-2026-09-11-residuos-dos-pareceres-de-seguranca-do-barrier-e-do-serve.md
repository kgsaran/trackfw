---
status: backlog
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-residuos-dos-pareceres-de-seguranca-do-barrier-e-do-serve.md"
squad: ""
---

# Roadmap: residuos dos pareceres de seguranca do barrier e do serve

> Created: 2026-09-11 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-11-residuos-dos-pareceres-de-seguranca-do-barrier-e-do-serve.md -->
REQ: docs/req/REQ-2026-09-11-residuos-dos-pareceres-de-seguranca-do-barrier-e-do-serve.md

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

### ML-1A — **AC1** — R1: VerifiesPassedBuffer portado para Node e Python, com a mesma conclusão afirmada.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — R1: VerifiesPassedBuffer portado para Node e Python, com a mesma conclusão afirmada.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — R2: guarda **estrutural** contra leitura duplicada no caller, nos 3 CLIs. 🔴 E a
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — R2: guarda **estrutural** contra leitura duplicada no caller, nos 3 CLIs. 🔴 E a
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — R3: cenários 8-9 emitem mensagem que **nomeia a causa**, sem perder o RC.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — R3: cenários 8-9 emitem mensagem que **nomeia a causa**, sem perder o RC.
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — R4: o gate invalida o pycache antes de medir. 🔴 **Falsificação:** com .pyc válido e
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — R4: o gate invalida o pycache antes de medir. 🔴 **Falsificação:** com .pyc válido e
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — contra-braço em cada um: configuração correta ⇒ passa. Guarda que só reprova é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — contra-braço em cada um: configuração correta ⇒ passa. Guarda que só reprova é
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **varredura:** o R4 é isolado? Outros gates nossos importam módulo Python cujo
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **varredura:** o R4 é isolado? Outros gates nossos importam módulo Python cujo
- [ ] build passes
- [ ] tests green
