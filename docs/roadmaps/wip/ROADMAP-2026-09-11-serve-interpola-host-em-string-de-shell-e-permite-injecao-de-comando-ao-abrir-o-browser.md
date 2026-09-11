---
status: wip
date: 2026-09-11
req: "docs/req/REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md"
squad: ""
---

# Roadmap: `serve` interpola `--host` em string de shell e permite injeção de comando ao abrir o browser

> Created: 2026-09-11 | Status: wip

## Context
<!-- Derived from REQ: REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md -->
REQ: docs/req/REQ-2026-09-01-serve-interpola-host-em-string-de-shell-e-permite-injecao-de-comando-ao-abrir-o-browser.md

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

### ML-1A — **AC1** — Nenhum caminho interpola valor controlável em string de shell. Use **argv** —
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — Nenhum caminho interpola valor controlável em string de shell. Use **argv** —
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — 🔴 O ramo Windows do Python (Popen(["start", url], shell=True)) deixa de usar
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — 🔴 O ramo Windows do Python (Popen(["start", url], shell=True)) deixa de usar
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 **Falsificação nas duas direções.** (a) o payload acima **não** executa comando
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 **Falsificação nas duas direções.** (a) o payload acima **não** executa comando
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — **Validar o --host na entrada**, não só escapar na saída. Um host que não é
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — **Validar o --host na entrada**, não só escapar na saída. Um host que não é
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Gate falsificável cobrindo AC1 e AC2 nos 3 CLIs. **Nasce ligado ao Makefile, com
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Gate falsificável cobrindo AC1 e AC2 nos 3 CLIs. **Nasce ligado ao Makefile, com
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — Paridade: os 3 CLIs abrem o browser pela mesma forma, e o contrato entra em
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — Paridade: os 3 CLIs abrem o browser pela mesma forma, e o contrato entra em
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — make quality e **CI** verdes.
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — make quality e **CI** verdes.
- [ ] build passes
- [ ] tests green
