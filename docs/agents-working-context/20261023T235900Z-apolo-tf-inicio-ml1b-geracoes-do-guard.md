# 20261023T235900Z — apolo-tf — INÍCIO: ML-1B (REQ-2026-10-23)

**Agente:** apolo-tf  
**Worktree:** `/private/tmp/trackfw-toml-escape`  
**Branch:** `fix/update-nao-remove-geracoes-antigas-do-credential-guard`  
**Roadmap:** `ROADMAP-2026-10-23-update-nao-remove-geracoes-antigas-do-credential-guard-em-pares-evento-matcher-e-o-validate-manda-rodar-update-que-nao-resolve.md`  
**Status anterior:** ML-1A ✅ Concluído (commit f3d171036e)

## Escopo de ML-1B

1. Implementar `migrateDedupGuardInNestedArray` e `removeGuardFromNestedArrayInHooks` em `internal/generators/guard_generations.go`
2. Implementar `dedupFlatGuardEntries` (variante sem append para Windsurf)
3. Corrigir comentário incorreto sobre Amazon Q usar formato flat (usa nested)
4. Modificar `InjectClaudeHooks`, `InjectCodexHooks`, `InjectGeminiHooks` com D13 dedup + D14 revised
5. Modificar `InjectWindsurfHooks` com dedup + D14 removal para credential guard
6. Modificar `InjectAmazonQHooks` com dedup para execute_bash/fs_write
7. Adicionar dedup nos harnesses (harnessCredentialGuardTargetClaude, etc.)
8. Testes: AC2, AC3, AC4, AC5, AC6, AC7 para sítios nested

**D14 revised (por par):**
- Claude: shell pair — remove quando global; Read/Write sempre presentes
- Codex: Bash pair — remove quando global; apply_patch sempre presente
- Gemini: run_shell_command — remove quando global; read/write sempre presentes
- Cursor: já implementado em ML-1A
- Windsurf: pre_run_command + pre_write_code — remove quando global; só credential
- Amazon Q: sem global, sem D14
