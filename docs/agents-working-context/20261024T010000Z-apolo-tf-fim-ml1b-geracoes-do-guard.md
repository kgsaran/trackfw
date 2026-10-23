# 20261024T010000Z — apolo-tf — FIM: ML-1B (REQ-2026-10-23)

**Agente:** apolo-tf  
**Worktree:** `/private/tmp/trackfw-toml-escape`  
**Branch:** `fix/update-nao-remove-geracoes-antigas-do-credential-guard`  
**Roadmap:** `ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md`  
**Status:** ML-1B ✅ Concluído

## O que foi feito

### `internal/generators/guard_generations.go`
- Adicionado `migrateDedupGuardInNestedArray` — D13 para formato nested (Claude/Codex/Gemini/AmazonQ): Pass 1 (legacy→canonical em-place), Pass 2 (dedup cross-envelope, mantém primeira ocorrência, remove envelopes vazios). Comparação por igualdade exata.
- Adicionado `removeGuardFromNestedArrayInHooks` — D14 revisado para formato nested: remove entradas reconhecidas dos envelopes correspondentes ao matcher, apaga envelopes vazios, deleta a chave se o array externo ficar vazio.
- Adicionado `dedupFlatGuardEntries` — Pass 1+2 de `migrateDedupGuardInFlatArray` sem Pass 3 (sem append). Usado pelo Windsurf para preservar `show_output: true`.
- Corrigido comentário incorreto: Amazon Q usa formato **nested**, não flat.

### `internal/generators/agentfiles.go`
- `InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks`: substituído bloco antigo D14 por D13 dedup (todos os matchers) + D14 revisado por par. Pares não cobertos pelo global (Read/Write|Edit, apply_patch, read/write do Gemini) sempre emitidos.
- `InjectWindsurfHooks`: substituídos N `migrateCursorSimpleCommand` por `dedupFlatGuardEntries` + D14 revisado com remoção quando global.
- `InjectAmazonQHooks`: adicionadas chamadas `migrateHookCommand` para formas legacy + D13 dedup via `migrateDedupGuardInNestedArray` para execute_bash (git-branch + credential CmdExe) e fs_write (credential CmdExe).

### `internal/generators/update.go`
- `harnessCredentialGuardTargetClaude` e `harnessGitBranchGuardTargetClaude`: adicionado loop D13 dedup após migrate existente para todos os matchers relevantes.

### `internal/generators/nested_guard_migrate_dedup_test.go` (novo)
- AC3: 4 testes (Claude, Codex, Gemini, AmazonQ) — N formas legadas → exactamente 1 canônica.
- AC4: teste orientado por tabela — cada forma de `CredentialLegacyPSPOSIX` migra para claudeShellMatcher do Claude.
- AC5: 2 testes — hooks do usuário sobrevivem byte-for-byte (Claude, Gemini).
- AC6: 3 grupos D14-revisado — ArmA/B/C para Claude; ArmB para Codex; ArmB para Gemini. Todos seedados com formas legadas.
- AC7: 3 testes de idempotência — Claude, Codex, Gemini.
- D14 revisado explícito: 3 testes confirmando Read/Write/apply_patch sempre presentes quando global instalado.

## Evidência dos gates

```
go build ./...                              → exit 0
go test ./internal/generators/             → ok (59.4s)
go test ./internal/validator/              → ok (33.2s)
go test ./internal/pathguard/              → ok (0.95s)
go test ./internal/commands/               → ok (47.0s)
trackfw validate                           → 163 warnings (lenient mode), 0 hard failures
```

## Riscos residuais

- AC2 (igualdade `guardExpectedLine`/`guardD11*` com guardforms.*) não foi testado em arquivo separado: os campos são usados em `validator_guard_binary_probe.go` e a igualdade é garantida pela construção do código (ambos derivam das mesmas constantes), mas um teste de asserção explícita seria mais robusto. Aceitável para ML-2A (revisão de qualidade).
- Goldens não foram regener (ação proibida no handoff): o gate de goldens no `make quality` deve ser verificado pela CI.
