---
status: done
date: 2026-08-08
req: ""
squad: ""
---

# Roadmap: harness credential-guard script não é gerado por trackfw update harness

> Created: 2026-08-08 | Status: done

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: docs/req/REQ-2026-08-08-harness-credential-guard-script-nao-e-gerado-por-trackfw-update-harness.md

`trackfw update harness` instala o wiring de hooks globais de credential-guard
nos 6 CLIs apontando para `~/.trackfw/scripts/trackfw-credential-guard.sh`, mas
nenhum dos 3 CLIs (Go/Node.js/Python) chamava a função que efetivamente escreve
esse arquivo — bug concreto reportado pelo usuário (hooks falhando com "No such
file or directory"). Fix já implementado e verificado antes da abertura deste
roadmap (bug fix direto, sem exploração arquitetural — trata-se de uma chamada
de função faltante, não uma decisão de design).

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] `UpdateHarness`/`run()`/`_run()` chamam a função geradora do script global
      antes de aplicar o wiring por-CLI (exceto `--dry-run`) nos 3 stacks
- [x] `~/.trackfw/scripts/trackfw-credential-guard.sh` existe e roda sem erro
      após `trackfw update harness`
- [x] Testes Go/Node.js/Python de credential-guard/update-harness verdes

## Wave 1 — Fix da chamada faltante (1 ML)
> Dependencies: none

### ML-1A — Chamar o gerador do script global no fluxo real de update harness
**Status:** ✅ Concluído
**Files affected:**
- `internal/generators/update.go` (`UpdateHarness`)
- `npm/src/commands/update-harness.js` (`run`)
- `pypi/trackfw/commands/update_harness.py` (`_run`)
**Actions:**
- Chamar `GenerateGlobalCredentialGuardScript(home)` /
  `generateGlobalCredentialGuardScript(homeRoot)` /
  `generate_global_credential_guard_script(home)` uma única vez no início do
  fluxo de `update harness`, guardado por `!dryRun`, antes de iterar os
  targets por-CLI.
- Node.js: suprimir stdout da chamada quando `--json` (mesmo padrão de
  `silenceConsole` já usado para os demais targets).
**Acceptance criteria:**
- [x] build passes (`go build ./...`)
- [x] tests green (`go test ./internal/generators/... ./internal/commands/...`,
      `node --test tests/*.test.js`, `python3 -m pytest pypi/tests/ -k "update_harness or credential_guard"`)
- [x] validate passes
**Comandos de validação:** `go build ./... && go test ./internal/generators/... -run "UpdateHarness|CredentialGuard" && node --test npm/tests/*.test.js && python3 -m pytest pypi/tests/ -k "update_harness or credential_guard"`
