---
status: backlog
date: 2026-09-11
req: "docs/req/REQ-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md"
squad: ""
---

# Roadmap: o ciclo testa onde funciona: faltam cp1252, Windows sem privilegio e consumidor novo

> Created: 2026-09-11 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md -->
REQ: docs/req/REQ-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md

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

### ML-1A — **AC1 — console cp1252.** Um passo de CI em windows-latest roda a suíte de gates com
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1 — console cp1252.** Um passo de CI em windows-latest roda a suíte de gates com
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2 — Windows SEM Developer Mode.** Um job que exercite o caminho de os.Symlink **sem** o
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2 — Windows SEM Developer Mode.** Um job que exercite o caminho de os.Symlink **sem** o
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3 — consumidor novo.** Um smoke que faz init num **projeto descartável**, com
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3 — consumidor novo.** Um smoke que faz init num **projeto descartável**, com
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4 — 🔴 job verde com anotação de erro reprova.** O #319 é um job success com 10
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4 — 🔴 job verde com anotação de erro reprova.** O #319 é um job success com 10
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5 — re-mutação de gate antigo.** Alvo (à la check-required-full) que aplique mutação nos
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5 — re-mutação de gate antigo.** Alvo (à la check-required-full) que aplique mutação nos
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6 — 🔴 cada gate novo precisa de contra-braço.** Ambiente que nunca reprova é indistinguível
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6 — 🔴 cada gate novo precisa de contra-braço.** Ambiente que nunca reprova é indistinguível
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7 — custo declarado.** Estes jobs acrescentam tempo a todo PR. **Meça e escreva** o custo;
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7 — custo declarado.** Estes jobs acrescentam tempo a todo PR. **Meça e escreva** o custo;
- [ ] build passes
- [ ] tests green
