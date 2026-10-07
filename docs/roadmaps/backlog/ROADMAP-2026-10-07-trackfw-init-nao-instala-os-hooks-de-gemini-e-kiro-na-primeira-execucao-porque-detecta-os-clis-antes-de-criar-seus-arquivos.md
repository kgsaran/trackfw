---
status: backlog
date: 2026-10-07
req: "docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md"
squad: ""
---

# Roadmap: trackfw init nao instala os hooks de Gemini e Kiro na primeira execucao porque detecta os CLIs antes de criar seus arquivos

> Created: 2026-10-07 | Status: backlog

## Context
<!-- Derived from REQ: REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md -->
REQ: docs/req/REQ-2026-10-07-trackfw-init-nao-instala-os-hooks-de-gemini-e-kiro-na-primeira-execucao-porque-detecta-os-clis-antes-de-criar-seus-arquivos.md

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
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
exit 1  # placeholder gate fails closed until ML-0A replaces it — see docs/cli-parity.md
```

## Wave 1 — Implementation (derived from REQ criteria)
> Dependencies: none
