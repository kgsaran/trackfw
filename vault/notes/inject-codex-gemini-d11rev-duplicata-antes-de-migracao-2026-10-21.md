# InjectCodexHooks/InjectGeminiHooks: merge antes da migração duplica o guard de credencial

Data: 2026-10-21 | REQ: REQ-2026-10-20 | Achado B-1 do red-team (hades-tf), confirmado pelo arquiteto

## Causa
Em `InjectCodexHooks` e `InjectGeminiHooks` (`internal/generators/agentfiles.go`), o `mergeClaudeHookArray` do guard de credencial roda ANTES do `migrateHookCommand` das formas legadas (D2/D11/D11-revisada/wrapper `/bin/bash -c`). O merge não acha a forma canônica e acrescenta uma entrada; depois a migração reescreve a legada para a canônica. Resultado: duas entradas idênticas no mesmo evento/matcher. O `InjectClaudeHooks` está correto (migra antes de mesclar).

## Medido
Configs versionados regenerados no ML-1A: `.codex/hooks.json` duplicado em PreToolUse[Bash] e [apply_patch]; `.gemini/settings.json` em BeforeTool[run_shell_command], [read_file|read_many_files], [write_file|replace]. `.claude/settings.json` sem duplicata. A migração do `~/.codex/hooks.json` global passa por outro caminho e não duplicou (medido no binário).

## Armadilha de medição
Contar duplicatas por chave truncada (primeiros 60 caracteres do comando) junta guards diferentes, porque o prefixo da linha polyglot é comum a todos. Compare o comando inteiro.

## Regra
Em todo `Inject*Hooks`: todas as migrações primeiro, depois os merges. Teste de regressão: arquivo com a forma legada → exatamente 1 entrada canônica por matcher.
