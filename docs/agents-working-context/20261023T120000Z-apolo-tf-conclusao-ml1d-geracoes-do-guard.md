# 2026-10-23 — apolo-tf — FIM: ML-1D corretivo (REQ-2026-10-23)

- Branch: `fix/update-nao-remove-geracoes-antigas-do-credential-guard`
- Worktree: `/private/tmp/trackfw-toml-escape`

## Entregáveis

**Task 1 — `TestNestedMigrate_AmazonQ_NLegacies_Deduplicates`:**
- Arquivo: `internal/generators/nested_guard_migrate_dedup_test.go`
- Adicionado loop após `n != 1` que verifica `countNestedEntries(..., legacy) == 0` para
  cada forma do slice `legacies`.
- Reconciliação atualizada: explica que a verificação de count==1 era insuficiente porque
  origin/main satisfazia essa condição ao adicionar 1 canônico sem remover as legadas.
- Reprova em origin/main com duas linhas FAIL (uma por forma semeada).

**Task 2 — `guard_registry_sites_ac4_test.go` (novo arquivo):**
- Arquivo: `internal/generators/guard_registry_sites_ac4_test.go`
- Teste `TestAC4_RegistrySites`: 23 linhas de tabela cobrindo 6 sítios de projeto
  (InjectClaude/Codex/Gemini/Cursor/Windsurf/AmazonQ) e 11 sítios de harness global
  (Claude/Codex/Gemini credential+git, Cursor credential+git, Copilot credential+git,
  Windsurf credential).
- Por linha: itera cada forma do registro referenciado, semeia legacy+canonical, roda a
  função real, afirma count(canonical)==1 e count(legacy)==0.
- Reprova em origin/main com 169 linhas FAIL.

## Falsificação
- Adicionado "trackfw guard credential # fictícia" a `guardforms.CredentialLegacyPSPOSIX`.
- Resultado: todos os subtestes PASSAM — porque todos os sítios referenciam a variável
  diretamente e o helper de dedup inclui a forma nova automaticamente.
- Reverted: `git diff internal/guardforms/` vazio após desfazer.

## Gates
- `go build ./...` — rc=0
- `go test ./internal/generators/ ./internal/validator/ ./internal/pathguard/ -count=1` — PASS
  (generators: 67.6 s, validator: 29.7 s, pathguard: 0.9 s)
- `trackfw validate` — 163 warnings, 0 errors (LENIENT MODE)

## ML-1D marcado ✅ Concluído no roadmap.
