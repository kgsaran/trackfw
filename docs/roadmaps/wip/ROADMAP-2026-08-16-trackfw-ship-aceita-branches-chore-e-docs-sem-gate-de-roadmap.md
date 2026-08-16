---
status: wip
date: 2026-08-16
req: ""
squad: ""
---

# Roadmap: trackfw ship aceita branches chore e docs sem gate de roadmap

> Created: 2026-08-16 | Status: wip

## Context

REQ: `docs/req/REQ-2026-08-16-trackfw-ship-aceita-branches-chore-e-docs-sem-gate-de-roadmap.md`

Fecha o par do #177. A release **7.0.0 está commitada e impublicável** até este fix entrar.

Pontos exatos no código: `internal/commands/ship.go:171` (mensagem) e `:513` (`isShipBranch`);
espelhos em `npm/src/commands/ship.js` e `pypi/trackfw/commands/ship.py` (textos de `--help` em
`ship.js:12` e `ship.py:18`).

<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — trackfw ship aceita branches chore e docs sem gate de roadmap
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes
