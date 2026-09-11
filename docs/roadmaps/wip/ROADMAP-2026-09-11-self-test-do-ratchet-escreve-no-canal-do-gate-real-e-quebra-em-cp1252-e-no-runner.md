---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md"
squad: ""
---

# Roadmap: self-test do ratchet escreve no canal do gate real e quebra em cp1252 e no runner

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md -->
REQ: docs/req/REQ-2026-09-11-self-test-do-ratchet-escreve-no-canal-do-gate-real-e-quebra-em-cp1252-e-no-runner.md

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

### ML-1A — **AC1** — 🔴 medir se a causa é comum **antes** de corrigir; a decisão de agrupar ou separar
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — 🔴 medir se a causa é comum **antes** de corrigir; a decisão de agrupar ou separar
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — o --self-test roda até o fim com sys.stdout.encoding = cp1252. 🔴 **Provado com o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — o --self-test roda até o fim com sys.stdout.encoding = cp1252. 🔴 **Provado com o
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — saída de **fixture** do self-test não vira anotação do runner. O job deixa de exibir
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — saída de **fixture** do self-test não vira anotação do runner. O job deixa de exibir
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — 🔴 **a distinção não pode custar o sinal real**: quando o gate **de produção** emite
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — 🔴 **a distinção não pode custar o sinal real**: quando o gate **de produção** emite
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — guarda de não-vacuidade: se o self-test deixar de exercitar os caminhos de erro para
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — guarda de não-vacuidade: se o self-test deixar de exercitar os caminhos de erro para
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — varredura dos **outros** check-*.sh com self-test: quantos têm o mesmo defeito?
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — varredura dos **outros** check-*.sh com self-test: quantos têm o mesmo defeito?
- [ ] build passes
- [ ] tests green
