# Claude Code: matcher "Bash" não cobre a ferramenta "PowerShell" no Windows

**Data:** 2026-10-07  
**Causa raiz de:** REQ-2026-09-05, Wave 5 (ML-5A)  
**Branch:** `docs/hooks-de-guard-executam-no-windows-prova-por-cli`

---

## O defeito

O Claude Code 2.1.292 no Windows expõe a ferramenta de execução de shell como `"PowerShell"` (não `"Bash"`). O campo `matcher` nos hooks de `.claude/settings.json` e `~/.claude/settings.json` é uma regex aplicada contra esse nome de ferramenta.

`trackfw init` emitia `"matcher": "Bash"` para todos os hooks de guard (git-branch e credential). Em Windows, o regex `Bash` não casa `PowerShell` — o hook nunca dispara, e `git push` executa sem bloqueio.

A doc oficial (docs.anthropic.com/en/docs/claude-code/hooks) confirma: no Windows, a ferramenta é `PowerShell` por padrão quando o Git Bash não está instalado, e `Bash` quando está.

## A correção

Constante única `claudeShellMatcher = "Bash|PowerShell"` em `internal/generators/agentfiles.go`. Todas as emissões de guard para Claude Code usam essa constante — nenhuma string literal `"Bash"` isolada para guards.

Migração automática: `migrateGuardHookMatcher` em `InjectClaudeHooks` e nos blocos de harness existente detecta o matcher antigo `"Bash"` em blocos de guard e renomeia para `claudeShellMatcher`.

**Codex:** mantém `"Bash"` — o Codex não usa o campo `matcher` no mesmo sentido; a ferramenta de shell do Codex não tem nome de dispatch idêntico ao do Claude Code.

## Sinal de alerta no validate

`validateClaudeGuardHookMatcherWarningsInFile` emite `credential_guard_hook_resolvable` (warning, não violation) quando um hook de guard do Claude Code tem matcher que não cobre `PowerShell`. O validate produz o aviso nomeando o arquivo.

## Regra que permanece

Todo novo guard emitido para Claude Code deve usar `claudeShellMatcher`, não `"Bash"`. Qualquer teste que constrói fixture com `"Bash"` para guard do Claude Code deve ser atualizado para `claudeShellMatcher`.

## Inventário de outros CLIs (sem correção nesta REQ)

| CLI | Shell no Windows | Comando gerado por trackfw | Funciona no Windows sem Git Bash? |
|---|---|---|---|
| Codex | PowerShell | string nua via campo `command` | sim — se trackfw no PATH |
| Gemini | PowerShell (`ComSpec`→`pwsh`→`powershell`) | `"command": "bash scripts/..."` | não — precisa de bash no PATH |
| Kiro | `cmd.exe` | `"command": "bash scripts/..."` | não — cmd.exe não tem bash |
| Copilot | PowerShell (campo `powershell`) | só campo `"bash"` emitido | não — campo `bash` ignorado no Windows |
| Cursor | PowerShell | wrapper PS com `$input \| <command>` | sim — PowerShell resolve trackfw no PATH |
| Windsurf | PowerShell (campo `command` fallback) | `"command": "bash scripts/..."` | não — `bash` não existe no PATH sem Git Bash |
| Amazon Q | `cmd.exe` | campo `command` com script bash | não — cmd.exe não executa bash nativo |

Fonte: `docs/portabilidade/2026-10-04-remedicao-do-schema-de-hook-dos-clis-de-agente.md`
