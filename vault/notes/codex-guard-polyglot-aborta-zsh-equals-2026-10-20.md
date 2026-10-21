# Linha polyglot de guard aborta no zsh antes do binário

Data: 2026-10-20

## Evidência

As três constantes guard*CmdPSPOSIX em internal/generators/agentfiles.go:503–505 começam com `$LASTEXITCODE=2`. Em zsh com variável vazia, a palavra se torna `=2`; a expansão EQUALS tenta resolver comando 2 e aborta o shell antes do guard, com exit 1 e stderr `zsh:1: 2 not found`. A mesma linha passa no Bash. A semente isolada `=2 2>/dev/null; printf reached` comprova: zsh não imprime, Bash imprime.

## Lacuna anterior

A matriz da ADR-2026-10-04 cobria sh/bash, Git Bash, PowerShell e cmd, mas não zsh. Testar a linha só em Bash não comprova execução pelo runtime macOS com SHELL=/bin/zsh. Os defeitos dos hooks personalizados rewriter/guardrails eram independentes e não explicavam os avisos repetidos de exit 1.

## Mitigação instalada

Nove entradas (4 globais, 5 projeto) foram envolvidas em `/bin/bash -c` com argumento literal cotado, sem alterar lógica do guard ou hooks personalizados. Backup de ambos JSONs e manifesto em ~/.codex/hooks/backups/20261020T110406812571Z/. Revisão independente aprovada. Três testes passam, incluindo binário ausente=2 e deny=2. Verificação pós-instalação: 18 execuções zsh -c/-lc retornam 0 para payload benigno; arquivos idênticos aos auditados.

Workaround restrito à instalação local macOS; não é contrato multiplataforma. Atualização que reinstale linhas canônicas pode recriar o defeito até a correção do gerador. REQ-2026-10-20 e roadmap pareado foram criados no backlog. Não desativar guards nem tratar exit 1 como aprovação.
