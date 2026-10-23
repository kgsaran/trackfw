# 20261023T180000Z — hades-tf — FIM: ML-0A Wave 0 (REQ-2026-10-23 gerações do guard)

- Entregável: `docs/seguranca/2026-10-23-wave0-geracoes-do-guard.md`
- Gates do roadmap: todos 3 verdes (arquivo não-vazio, contém 'Matriz', `git diff --quiet origin/main -- internal/` rc=0)
- Worktree: `/private/tmp/trackfw-toml-escape`. Branch: `fix/update-nao-remove-geracoes-antigas-do-credential-guard`

Achados principais:

1. **Cursor preToolUse×Read/Write e postToolUse×Read/Write: não migra.** `mergeCursorGuardMatcherEntry` acrescenta a forma canônica sem remover legadas. Medido: com 5 legadas em `preToolUse×Read`, resultado = 6 entradas (5 legadas + 1 canônica). `origin/main` confirmado. AC1 exige 1 entrada por par.

2. **Cursor beforeShellExecution com N legadas: migra e duplica.** `migrateCursorSimpleCommand` reescreve cada legada para canônica; o merge não deduplica. Medido: 5 legadas → 6 entradas (5 canônicas + 1 git-branch). AC1 exige deduplicação após migração.

3. **D14 gap: com global instalado, preToolUse×Read/Write ficam orfanados.** InjectCursorHooks pula os quatro `mergeCursorGuardMatcherEntry` quando `globalCredentialGuardInstalledCursor()` retorna true. O harness global só cobre `beforeShellExecution`/`afterShellExecution`. Legadas em `preToolUse×Read` não são removidas. Medido pelo cenário D14.

4. **Gemini usa string literal** `"scripts/trackfw-credential-guard.sh"` em linhas 847–858 de `agentfiles.go` em vez de `legacyBareCredGuardCmd`. AC2 deve cobrir esse caso.

5. **Windsurf e AmazonQ: credential guard de projeto sem migração de legadas.** Mesma causa; entram como ML adicional no ML-1A pela Regra Dura.

6. **D2 cmd.exe = comando mínimo do usuário.** Remoção por igualdade exata aceita como residual R3.

Nenhuma linha de produto escrita. Checkout limpo confirmado.
