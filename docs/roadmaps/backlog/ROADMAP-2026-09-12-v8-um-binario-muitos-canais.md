---
status: backlog
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-v8-um-binario-muitos-canais-casquinha-npm-e-wheels-de-plataforma-substituem-as-reimplementacoes-node-e-python.md"
squad: ""
---

# Roadmap: v8 um binario muitos canais

> Created: 2026-09-12 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-09-12-v8-um-binario-muitos-canais-casquinha-npm-e-wheels-de-plataforma-substituem-as-reimplementacoes-node-e-python.md

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

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — v8 um binario muitos canais
**Status:** ⬜ Pendente
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
