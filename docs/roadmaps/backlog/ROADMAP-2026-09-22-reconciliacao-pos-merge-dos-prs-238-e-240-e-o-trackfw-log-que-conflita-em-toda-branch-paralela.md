---
status: backlog
date: 2026-09-22
req: "docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md"
squad: ""
---

# Roadmap: Reconciliação pós-merge dos PRs #238 e #240, e o `.trackfw-log` que conflita em toda branch paralela

> Created: 2026-09-22 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md -->
REQ: docs/req/REQ-2026-09-02-reconciliacao-pos-merge-dos-prs-238-e-240-e-o-trackfw-log-que-conflita-em-toda-branch-paralela.md

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

### ML-1A — **AC1** — errors="replace" vira strict **ou** o motivo de mantê-lo está escrito no código
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC1** — errors="replace" vira strict **ou** o motivo de mantê-lo está escrito no código
- [ ] build passes
- [ ] tests green

### ML-1B — **AC2** — A entrada de allowlist de check-output-encoding-declared.sh é removida **ou** seu
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC2** — A entrada de allowlist de check-output-encoding-declared.sh é removida **ou** seu
- [ ] build passes
- [ ] tests green

### ML-1C — **AC3** — 🔴 O aviso de "allowlist obsoleta" passa a reconhecer **também** sys.stdout.reconfigure
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC3** — 🔴 O aviso de "allowlist obsoleta" passa a reconhecer **também** sys.stdout.reconfigure
- [ ] build passes
- [ ] tests green

### ML-1D — **AC4** — O item 3 é **medido**: enumerar onde saída de gate é hasheada ou comparada byte a
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC4** — O item 3 é **medido**: enumerar onde saída de gate é hasheada ou comparada byte a
- [ ] build passes
- [ ] tests green

### ML-1E — **AC5** — Os roadmaps trazidos pelos PRs do reporter estão em done, e a política de quem faz
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC5** — Os roadmaps trazidos pelos PRs do reporter estão em done, e a política de quem faz
- [ ] build passes
- [ ] tests green

### ML-1F — **AC6** — .gitattributes com merge=union para .trackfw-log existe **neste** repositório
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC6** — .gitattributes com merge=union para .trackfw-log existe **neste** repositório
- [ ] build passes
- [ ] tests green

### ML-1G — **AC7** — 🔴 **Controle do AC6:** o merge=union **não** engole conteúdo — as linhas dos dois
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] **AC7** — 🔴 **Controle do AC6:** o merge=union **não** engole conteúdo — as linhas dos dois
- [ ] build passes
- [ ] tests green
