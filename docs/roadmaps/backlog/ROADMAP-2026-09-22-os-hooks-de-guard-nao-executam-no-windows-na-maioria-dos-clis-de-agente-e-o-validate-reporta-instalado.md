---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md"
squad: ""
---

# Roadmap: os hooks de guard nao executam no Windows na maioria dos CLIs de agente e o validate reporta instalado

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md -->
REQ: docs/req/REQ-2026-09-05-os-hooks-de-guard-nao-executam-no-windows-na-maioria-dos-clis-de-agente-e-o-validate-reporta-instalado.md

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

### ML-1A — **AC1** — **Copilot:** o hook passa a ser escrito no campo que o CLI lê no Windows.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — **Copilot:** o hook passa a ser escrito no campo que o CLI lê no Windows.
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — **Gemini e Codex:** hook que **executa** sob PowerShell.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — **Gemini e Codex:** hook que **executa** sob PowerShell.
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **A prova é o guard DISPARANDO e BLOQUEANDO**, não o arquivo existindo.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **A prova é o guard DISPARANDO e BLOQUEANDO**, não o arquivo existindo.
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **Paridade comportamental .sh ↔ nativo, por gate.** O git-branch-guard tem 561
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **Paridade comportamental .sh ↔ nativo, por gate.** O git-branch-guard tem 561
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — 🔴 **Controle POSIX:** Linux e macOS inalterados, medidos antes e depois.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — 🔴 **Controle POSIX:** Linux e macOS inalterados, medidos antes e depois.
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — 🔴 **Cursor e Kiro: NENHUMA emissão nova antes de medir.** Emitir para CLI cujo
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — 🔴 **Cursor e Kiro: NENHUMA emissão nova antes de medir.** Emitir para CLI cujo
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — 🔴 **O validate para de reportar como instalado o hook que não pode executar**
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — 🔴 **O validate para de reportar como instalado o hook que não pode executar**
- [ ] build passes
- [ ] tests green

### ML-1H — **AC8** — Regra dura de paridade: a mudança de emissão vale nos **3 CLIs** do trackfw
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC8** — Regra dura de paridade: a mudança de emissão vale nos **3 CLIs** do trackfw
- [ ] build passes
- [ ] tests green
