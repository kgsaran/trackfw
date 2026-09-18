---
status: done
date: 2026-08-16
req: ""
squad: ""
---

# Roadmap: trackfw ship aceita branches chore e docs sem gate de roadmap

> Created: 2026-08-16 | Status: done

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
**Status:** ✅ Concluído
**Files affected:**
- `internal/commands/ship.go`, `internal/commands/ship_test.go`
- `npm/src/ship/runner.js`, `npm/src/commands/ship.js`, `npm/tests/ship.test.js`
- `pypi/trackfw/ship/runner.py`, `pypi/trackfw/commands/ship.py`, `pypi/tests/test_ship.py`
- `scripts/check-ship-parity.sh` (novo), `Makefile`
- `docs/cli-parity.md`, `vault/notes/ship-checkgovernance-error-stream-wording-divergence-2026-08-16.md`, `vault/notes/index.md`
**Actions:**
- `isShipBranch`/`isGatedShipBranch` (e equivalentes JS/Python) separam vocabulário
  (feat/fix/refactor/chore/docs) do gate de governança (feat/fix/refactor apenas), espelhando
  `branchValidTypes`/`branchGatedTypes` de `branch.go`.
- Step 2 de `runShip` passa a pular `checkGovernance()` inteiramente para branch chore/docs,
  imprimindo `Governance: skipped (chore/docs branch)` — independente do conteúdo staged.
- Mensagem de branch inválida e `--help` (regras 1/2) atualizados, byte-idênticos nos 3 CLIs.
- Novo `scripts/check-ship-parity.sh` cobre os 2 cenários novos (chore/docs) com diff completo
  de stream, e 2 cenários de não-regressão (feat sem roadmap; branch fora do vocabulário) com
  asserções de conteúdo — ver nota do vault para o porquê da divergência pré-existente que
  motivou essa escolha.
**Acceptance criteria:**
- [x] build passes
- [x] tests green
- [x] validate passes
- [x] make quality verde
