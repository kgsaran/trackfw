# Contexto — apolo-tf — início do ML-1A (gerações do guard)

**Data:** 2026-10-23T20:00:00Z
**REQ:** REQ-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard...
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas...
**ML:** ML-1A — Registro único, deduplicação genérica e o Cursor (a issue)
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard

## O que foi recebido

Wave 0 aprovada (ML-0A, ML-0B). Matriz consolidada em §C do parecer de segurança.
`git diff --quiet origin/main -- internal/` → rc=0 (árvore limpa).

## Decisão de pacote (D12)

Confirmado ciclo de importação: `go list -deps ./internal/generators` inclui
`internal/validator`. Portanto o registro vai em `internal/guardforms/` (pacote folha,
sem importações internas).

Testes de concordância que escaneiam `agentfiles.go` serão atualizados para escanear
`guardforms/guard_generations.go` (TestGuardWiringConcordance_AllGeneratorFormsRecognized e
TestGuardWiringConcordance_LegacyD2D11FormsRecognized).

## Mudanças planejadas

- CREATE `internal/guardforms/guard_generations.go` — registry (D12)
- CREATE `internal/generators/guard_generations.go` — D13 helpers
- MODIFY `internal/generators/agentfiles.go` — aliases para guardforms; InjectCursorHooks com D13
- MODIFY `internal/validator/validator_guard_wiring.go` — guardWiringScriptPathForms via guardforms
- MODIFY `internal/validator/validator_guard_wiring_concordance_external_test.go` — scan guardforms
- MODIFY `internal/generators/credential_guard_dedup_test.go` — atualizar TestDedup_Cursor_SkipsProjectEntryWhenGlobalInstalled (D14 revista)
- CREATE `internal/generators/cursor_guard_migrate_dedup_test.go` — AC5, AC6, AC7
- CREATE `internal/validator/cursor_guard_migrate_dedup_external_test.go` — AC1, AC4
