---
name: winskips-three-bypass-patterns-ml-fa
description: winskips gate: tagless switch, local-var GOOS e continue são invisíveis — 3 padrões Go válidos não detectados pelo gate interno/winskips
metadata:
  type: project
---

Encontrado 2026-11-17, ML-FA Wave 5, REQ-2026-11-15.

Três padrões Go naturais e válidos (que linters Go não rejeitam) não são detectados pelo gate `internal/winskips`:

1. **Tagless switch** (`switch { case runtime.GOOS=="windows": t.Skip() }`) — `winskips.go:488`: `stmt.Tag == nil || !isGOOSSelector(stmt.Tag)` hits `break` sem analisar os cases.
2. **Local-var GOOS** (`goos := runtime.GOOS; if goos == "windows" { t.Skip() }`) — `isGOOSSelector` só reconhece `runtime.GOOS` literal, não ident de local var.
3. **Continue em branch Windows** (`for { if runtime.GOOS=="windows" { continue } }`) — `branchHasExit` verifica apenas `ReturnStmt` e `t.Skip*`.

**Medição:** probe em `winskips-probe/bypass_test.go` no scratchpad — 3 FAILs confirmados.
**Scan real:** zero ocorrências hoje na árvore (`grep -rn 'goos := runtime.GOOS'` → 0 hits). Riscos futuros.
**Governança:** nenhum dos três é declarado como residual em REQ, Roadmap ou Wave 0 (grep zero). BLOQUEANTE.
**Fix:** (1) ramo para `stmt.Tag == nil` em `nodeHasPlatformExit`; (2) pré-passo para local-var GOOS; (3) `BranchStmt{Tok: CONTINUE|BREAK}` em `branchHasExit`.
**4-A4 relacionado:** `evidence="PENDING"` aceito — confirma AC aberto de ML-3A2.

**Why:** winskips existe para garantir que toda cobertura Windows silenciada seja documentada; os 3 bypass fariam testes silenciados passarem sem documentação.
**How to apply:** ao auditar winskips em MLs futuros, executar probe similar; verificar se esses padrões foram corrigidos antes de aprovar.
