# apolo-tf — fim de sessão — ML-1C (REQ-2026-10-23)

**Timestamp:** 2026-10-23T15:00:00Z
**Branch:** fix/update-nao-remove-geracoes-antigas-do-credential-guard
**Roadmap:** ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md

## O que foi feito

ML-1C concluído: testes para a lacuna D13 do harness e para o contrato AC2 do validador.

### Arquivos escritos

**`internal/generators/guard_harness_dedup_ml1c_test.go`** (14 testes, package `generators`):
- `TestHarness_ML1C_Codex_Credential_NLegaciesDedup` — 4 legadas em ~/.codex/hooks.json → 1 canônica (PreToolUse × Bash, PostToolUse × Bash)
- `TestHarness_ML1C_Codex_GitBranch_NLegaciesDedup`
- `TestHarness_ML1C_Gemini_Credential_NLegaciesDedup` — BeforeTool/AfterTool × run_shell_command
- `TestHarness_ML1C_Gemini_GitBranch_NLegaciesDedup`
- `TestHarness_ML1C_Cursor_Credential_NLegaciesDedup` — flat {"command": ...}
- `TestHarness_ML1C_Cursor_GitBranch_NLegaciesDedup`
- `TestHarness_ML1C_Copilot_Credential_NLegaciesDedup` — flat {"bash": ...}
- `TestHarness_ML1C_Copilot_GitBranch_NLegaciesDedup`
- `TestHarness_ML1C_Windsurf_Credential_NLegaciesDedup` — global harness, todos CredentialGlobalLegacyPSPOSIX
- `TestAmazonQ_ML1C_GitBranch_PSPOSIXForms_Migrate` — GitBranchLegacyPSPOSIX em execute_bash → CmdExe canonical
- `TestWindsurf_ML1C_NCredentialLegacies_Deduplicates` — InjectWindsurfHooks project-scope
- `TestWindsurf_ML1C_NGitBranchLegacies_Deduplicates`
- `TestWindsurf_ML1C_AC5_UserHookSurvives`
- `TestWindsurf_ML1C_D14_GlobalInstalled_RemovesProjectCredential`

**`internal/validator/guard_ac2_lines_ml1c_test.go`** (4 funções de teste, 13 sub-testes, package `validator`):
- `TestAC2_GuardExpectedLine_MatchesGuardforms` (5 casos)
- `TestAC2_GuardD11RevisedLegacyLine_MatchesGuardforms` (3 casos)
- `TestAC2_GuardD11LegacyLine_MatchesGuardforms` (3 casos)
- `TestAC2_GuardD2LegacyLine_MatchesGuardforms` (2 casos)

### Gates

```
go build ./...                                                           rc=0
go test ./internal/generators/ ./internal/validator/
      ./internal/pathguard/ ./internal/commands/ -count=1               ok (4 pacotes)
trackfw validate                                                         exit=0 (163 warnings, 0 violations)
```

## Estado do roadmap

- ML-0A ✅, ML-0B ✅, ML-1A ✅, ML-1B ✅ (com nota de auditoria), ML-1C ✅
- Wave 2 (ML-2A hefesto-tf, ML-2B hades-tf) e Wave 3 (ML-3A zeus-tf) pendentes

## Próximo

Handoff ao arquiteto (zeus-tf) para auditoria e commit do ML-1C.
