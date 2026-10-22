# Linha polyglot de guard aborta no zsh antes do binário

Data: 2026-10-20

## Evidência

As três constantes guard*CmdPSPOSIX em internal/generators/agentfiles.go:503–505 começam com `$LASTEXITCODE=2`. Em zsh com variável vazia, a palavra se torna `=2`; a expansão EQUALS tenta resolver comando 2 e aborta o shell antes do guard, com exit 1 e stderr `zsh:1: 2 not found`. A mesma linha passa no Bash. A semente isolada `=2 2>/dev/null; printf reached` comprova: zsh não imprime, Bash imprime.

## Lacuna anterior

A matriz da ADR-2026-10-04 cobria sh/bash, Git Bash, PowerShell e cmd, mas não zsh. Testar a linha só em Bash não comprova execução pelo runtime macOS com SHELL=/bin/zsh. Os defeitos dos hooks personalizados rewriter/guardrails eram independentes e não explicavam os avisos repetidos de exit 1.

## Segundo breakpoint — NOMATCH no segmento 4 (adendo 2026-10-21)

O segmento 4 da linha (`$LASTEXITCODE=2*!!$LASTEXITCODE 2>...`) também aborta o zsh, mas por mecanismo diferente: após o segmento 3 setar LASTEXITCODE como variável de shell, o segmento 4 expande para `N=2*!!N`; o `*` sem aspas em posição de valor de atribuição é glob; com a opção NOMATCH do zsh (padrão ON), zero correspondências abortam com `zsh:1: no matches found: N=2*!!N`. O `!!` é literal em shell não-interativo (sem expansão de histórico). Um fix que desative apenas EQUALS ainda falha no segmento 4.

Candidato medido (Wave 0, 2026-10-21): prefixar `unsetopt EQUALS NOMATCH 2>${null-/dev/null}; ` à linha atual. Em sh/bash/dash, `unsetopt` é CommandNotFound (exit 127, stderr swallowed), shell continua. Em zsh, desabilita ambas as expansões antes de qualquer segmento. Medido em sh/bash/dash/zsh/zsh-f: todos os 4 casos (allow/deny/absent/err1) corretos. PS: não medido nesta etapa (R1 da Wave 0).

## Claude Code macOS também usa zsh (adendo 2026-10-21)

`$ZSH_VERSION=5.9` foi impresso pelo tool "Bash" da sessão Claude Code atual em macOS. O tool usa o SHELL do processo (zsh). Se os hooks definidos em `.claude/settings.json` são executados pelo mesmo mecanismo, a falha se replica para Claude Code em macOS. Não medido em processo de hook real — registrado como residual R2 na Wave 0. Blast radius potencial: Codex + Claude Code em macOS = todas as instalações com SHELL=/bin/zsh.

## Mitigação instalada

Nove entradas (4 globais, 5 projeto) foram envolvidas em `/bin/bash -c` com argumento literal cotado, sem alterar lógica do guard ou hooks personalizados. Backup de ambos JSONs e manifesto em ~/.codex/hooks/backups/20261020T110406812571Z/. Revisão independente aprovada. Três testes passam, incluindo binário ausente=2 e deny=2. Verificação pós-instalação: 18 execuções zsh -c/-lc retornam 0 para payload benigno; arquivos idênticos aos auditados.

Workaround restrito à instalação local macOS; não é contrato multiplataforma. Atualização que reinstale linhas canônicas pode recriar o defeito até a correção do gerador. REQ-2026-10-20 e roadmap pareado foram criados no backlog. Não desativar guards nem tratar exit 1 como aprovação.

## Correção da auditoria (zeus-tf, 2026-10-21)

O adendo que coloca o Claude Code no macOS executando hooks em zsh está **errado**. O `$ZSH_VERSION` veio do shell da ferramenta Bash. O executor de hooks do Claude roda a mesma linha polyglot e **negou** corretamente (`git worktree remove --force`, com a mensagem do guard) nesta sessão. Em zsh, teria saído 1. Medido como afetado: o Codex. Ainda no zsh: o deny também sai 1, ou seja, fail-open.
