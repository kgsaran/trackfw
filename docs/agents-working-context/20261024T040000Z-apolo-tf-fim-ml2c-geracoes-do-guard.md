# apolo-tf — fim ML-2C (2026-10-24T04:00:00Z)

**Agente:** apolo-tf
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md

## O que foi feito no ML-2C

### Gemini B1 (Hefesto)
Removidas 6 chamadas a `migrateHookCommand` com literal `"scripts/trackfw-credential-guard.sh"` em
`InjectGeminiHooks` (`internal/generators/agentfiles.go`). A deduplicação via `migrateDedupGuardInNestedArray`
já cobre `LegacyBareCredGuard` pelo registro. Grep pelo literal fora de testes: apenas `internal/guardforms`.

### Copilot B1 (Hades, D14 revisada)
`view` e `create|edit` (pre e post) movidos para fora do bloco `if !globalCredentialGuardInstalledCopilot()`.
O global do Copilot cobre apenas `bash`; os outros pares são sempre emitidos.

### Kiro (mesmo defeito D14)
`read` e `write` (pre e post, 4 entradas) movidos para fora do bloco `if !globalCredentialGuardInstalledKiro()`.
O global do Kiro cobre apenas `shell`; os outros pares são sempre emitidos.

### R1 — migrateDedupGuardInFlatArray
Reescrita para delegar Passes 1 e 2 a `dedupFlatGuardEntries`. Elimina ~55 linhas duplicadas.
Arquivo: `internal/generators/guard_generations.go`.

### R4 — copy-before-append
9 sítios em `internal/generators/update.go` + 1 em `agentfiles.go` corrigidos para
`append(append([]string{}, guardforms.X...), ...)`. Comentários `//nolint:gocritic` removidos.

### R2 — renomear testes _ML1C_
Novo arquivo `guard_harness_dedup_test.go` com nomes por comportamento.
Arquivo `guard_harness_dedup_ml1c_test.go` deletado.

### R3 — remover teste subsumed
`TestNestedMigrate_Claude_AC4_EveryRegistryFormMigrates` e helper `minInt` removidos de
`nested_guard_migrate_dedup_test.go`.

### Testes existentes atualizados
`credential_guard_dedup_test.go`: `TestDedup_Copilot_SkipsProjectEntryWhenGlobalInstalled` e
`TestDedup_Kiro_SkipsProjectEntryWhenGlobalInstalled` atualizados para refletir o novo comportamento D14.

### Novos testes D14
`copilot_kiro_d14_ml2c_test.go` criado com:
- `TestCopilotD14_ViewCreateEditAlwaysPresent` (2 arms: no-global, with-global)
- `TestKiroD14_ReadWriteAlwaysPresent` (2 arms: no-global, with-global)
- `TestKiroD14_ShellRemovalRequiresLegacySeed`

## Gates

```
go build ./...                                           rc=0
go test ./internal/generators/ ./internal/validators/ \
  ./internal/pathguard/ ./internal/commands/ -count=1
ok  github.com/kgsaran/trackfw/internal/generators    60.041s
ok  github.com/kgsaran/trackfw/internal/validator     32.473s
ok  github.com/kgsaran/trackfw/internal/pathguard      0.876s
ok  github.com/kgsaran/trackfw/internal/commands      46.648s
```

## Contra-braço

`TestCopilotD14_ViewCreateEditAlwaysPresent/with-global` → FAIL no HEAD anterior (pré-ML-2C)
`TestKiroD14_ReadWriteAlwaysPresent/with-global` → FAIL no HEAD anterior (pré-ML-2C)
Confirma: os testes novos reprovam a condição exata que o ML corrige.

## Estado atual

ML-2C ✅ Concluído. Próximo: ML-3A (zeus-tf: `make quality` + CI).

## git status --short (ao encerrar)

```
M  internal/generators/agentfiles.go
 M internal/generators/credential_guard_dedup_test.go
 M internal/generators/guard_generations.go
 D internal/generators/guard_harness_dedup_ml1c_test.go
 M internal/generators/nested_guard_migrate_dedup_test.go
 M internal/generators/update.go
?? internal/generators/copilot_kiro_d14_ml2c_test.go
?? internal/generators/guard_harness_dedup_test.go
M  docs/roadmaps/wip/ROADMAP-2026-10-23-...
```
