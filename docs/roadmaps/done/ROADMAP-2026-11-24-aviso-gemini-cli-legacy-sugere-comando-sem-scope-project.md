---
status: done
date: 2026-11-24
req: "docs/req/REQ-2026-11-24-aviso-gemini-cli-legacy-sugere-comando-sem-scope-project.md"
squad: ""
---

# Roadmap: Aviso gemini_cli_legacy sugere comando sem --scope project

> Created: 2026-11-24 | Status: done

## Context
<!-- Derived from REQ: REQ-2026-11-24-aviso-gemini-cli-legacy-sugere-comando-sem-scope-project.md -->
REQ: docs/req/REQ-2026-11-24-aviso-gemini-cli-legacy-sugere-comando-sem-scope-project.md

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [x] Mensagem com `--scope project` (issue #705)
- [x] Teste fixa o `--scope project`; `cli-parity.md` atualizado
- [x] Censo das mensagens de regra de projeto com comando de escopo
- [x] `make quality` rc=0 e CI verde

## Status Legend
⬜ Pending · 🔄 In progress · ✅ Done · ❌ Blocked

## Wave 0 — Threat Model
> Dependencies: none. Blocks all implementation.

### ML-0A — Threat model for this roadmap (hades-tf)
**Status:** ✅ Done — sítio único: `validator_gemini_cli_legacy.go:48`. S2–S7 foram classificados: condição global com
`update harness`, `trackfw update` (só de projeto) ou escopo interpolado no `doctor`. A correção silencia o aviso
(`InjectAntigravityHooks` grava `.agents/hooks.json`). Residual declarado: sem `--items`, o remédio instala os 12
agentes do catálogo no projeto, o que vira problema quando o ML-3A pendente de `integrations_flags.go:329` entrar.
**Files affected:** `docs/seguranca/2026-11-24-wave0-correcoes-de-consumidor-704-705.md` (novo; compartilhado com o roadmap irmão da mesma branch)
**Actions:**
1. Enumeration completeness — is the list of surfaces in this roadmap complete? Name what is missing, or show the list is closed. Do not limit the search to the files already named by the REQ — before declaring the list closed, search the repository for other places that emit the same artifact or the same pattern (for example, grep for the literal the final artifact contains).
2. Threat model — who empties this Wave 0 without breaking any written rule, and how?
3. Falsification targets in both directions — for each surface, what breaks when the behavior regresses, and what breaks when it regresses the opposite way?
4. Declared residual — what this design accepts not covering.
**Acceptance criteria:**
- [x] The four sections above answered with evidence, not a one-line assertion
- [x] No implementation line written for this ML

**Wave gates:**
```bash
# Wave 0 gate — replace this placeholder with a project-specific check before
# marking ML-0A done. Do not remove the gate; replace its command (AC13).
# each line runs as a separate sh -c — see docs/cli-parity.md rule 5
test -s docs/seguranca/2026-11-24-wave0-correcoes-de-consumidor-704-705.md
```

## Wave 1 — Implementation
> Dependencies: Wave 0 audited. Disjoint from the sibling roadmap (#704); one apolo dispatch covers both.

### ML-1A — Remédio do aviso com `--scope project` (apolo-tf)
**Status:** ✅ Done — mensagem e comentário com `--scope project`; `TestGeminiCliLegacy_GeminiGuardNoAntigravity_Warns`
afirma o texto (braço: rc=1 com a mensagem antiga); `cli-parity.md` L10470 atualizado.
**Files affected:** `internal/validator/validator_gemini_cli_legacy.go`;
`internal/validator/validator_guard_hook_concordance_external_test.go`; `docs/cli-parity.md`
**Actions:**
1. `validator_gemini_cli_legacy.go:48` (e o comentário da L10): `trackfw agents install --targets antigravity` →
   `trackfw agents install --targets antigravity --scope project`.
2. No teste existente do warning (`validator_guard_hook_concordance_external_test.go` ~L1110–1150), afirmar que o
   texto contém `--scope project`. Braço: reprova com a mensagem antiga.
3. `docs/cli-parity.md` ~L10470: mesma troca.
4. Se o Wave 0 apontar outra mensagem do mesmo tipo, PARE e reporte (o arquiteto acrescenta ao escopo).
**Acceptance criteria:**
- [x] `go test ./internal/validator/ ./internal/commands/ -count=1` rc=0; frase de reconciliação
- [x] nenhum arquivo fora de "Files affected" alterado

## Wave 2 — Review
> Dependencies: Wave 1 audited. Revisão e `make quality` compartilhados com o roadmap irmão (#704): ML-2A e ML-2B de
> lá cobrem este roadmap.

### ML-2A — `make quality` e CI (zeus-tf)
**Status:** ✅ Done — `make quality` rc=0; CI do PR #709: 24/24 SUCCESS (2026-11-25)
**Files affected:** este roadmap
**Acceptance criteria:**
- [x] `make quality` rc=0; CI verde; revisão do hefesto (roadmap irmão) sem achado bloqueante

**Wave gates:**
```bash
go build ./...
go test ./internal/validator/ -count=1
```
