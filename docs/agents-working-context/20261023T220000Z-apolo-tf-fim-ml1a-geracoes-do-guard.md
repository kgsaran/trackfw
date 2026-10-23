# Contexto — apolo-tf — fim do ML-1A (gerações do guard)

**Data:** 2026-10-23T22:00:00Z
**REQ:** REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard...
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas... (ML-1A ✅ Concluído)
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard

## O que foi entregue

### Novos arquivos criados
- `internal/guardforms/guard_generations.go` — registro D12: todas as constantes e listas de gerações conhecidas de credential e git-branch guard (PSPOSIX e CmdExe), consumível por generators e validator sem ciclo de importação.
- `internal/generators/guard_generations.go` — helpers D13: `migrateDedupGuardInFlatArray`, `removeGuardFromFlatArrayInHooks`, `migrateDedupGuardInNestedArray`.
- `internal/generators/cursor_guard_migrate_dedup_test.go` — AC5 (user hook sobrevive), AC6 (3 braços D14), AC7 (idempotência), AC7-legacy (migração de 5 gerações).
- `internal/validator/cursor_guard_migrate_dedup_external_test.go` — AC1 (fixture 5 gerações → 1 canônica reconhecida pelo validator), AC4 (concordância D12 ↔ validator, falsificação).

### Arquivos modificados
- `internal/generators/agentfiles.go` — import guardforms; `InjectCursorHooks` reescrita: migração+dedup atômica via helpers D13 para shell events (com remoção quando global instalado, D14 revista) e pairs Read/Write sempre emitidos (fora do if global).
- `internal/validator/validator_guard_wiring.go` — import guardforms; `guardWiringScriptPathForms` agora construído do registro via `buildGuardWiringScriptPathForms()` (tipo nomeado `guardWiringScriptPathEntry`).
- `internal/generators/credential_guard_dedup_test.go` — `TestDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled` atualizado para D14 revista (pre/post têm 3 entradas cada: signal/cleanup + Read guard + Write guard).

## Gates (evidência)

```
go build ./...       → rc=0 (sem saída)
go test ./internal/generators/ ./internal/validators/ ./internal/pathguard/ -count=1
→ ok generators 59.552s | ok validator 31.765s | ok pathguard 0.949s
```

## Riscos remanescentes / ML-1B pendente

ML-1B (Claude, Codex, Gemini, Windsurf, Amazon Q, harness targets) ainda usa `migrateCursorSimpleCommand` + `migrateHookCommand` em série ao invés dos helpers D13. Isso não é regressão (mesmo comportamento de antes desta REQ), mas significa que esses sítios ainda acumulam duplicatas se houver múltiplas gerações legadas. ML-1B deve ser despachado em handoff separado com o agente.

AC2 (cobertura parcial — Gemini literal e outros CLIs) continua como risco declarado até ML-1B.
