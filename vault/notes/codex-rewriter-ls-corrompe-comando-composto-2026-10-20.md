# Rewriter global do Codex corrompe comandos compostos iniciados por ls

Data: 2026-10-20

O script ~/.codex/hooks/pretooluse-rewriter.sh testa ^ls e concatena --color=never à string inteira. A entrada `ls /tmp; git status --short` resulta em `ls /tmp; git status --short --color=never`. Reproduzido executando o script com payload PreToolUse/Bash via subprocess: saída JSON updatedInput contém o comando corrompido; git status falha com unknown option color=never.

O hook retorna exit 0: um hook que executa com sucesso pode quebrar a ferramenta subsequente. Não aplicar reescrita textual de comando simples a uma lista shell. O script fica fora do repositório e não foi alterado neste diagnóstico.

## Correção instalada

Após autorização, os dois hooks personalizados foram corrigidos: rewriter limita reescrita a comandos simples com allowlist conservadora, preserva newlines e deixa ls/go build intactos; guardrails usa permissionDecision deny em vez de block. Cinco testes com subcasos passam nos arquivos instalados. Backup em ~/.codex/hooks/backups/20261020T105943314518Z/.

Origem: agentfiles.go:317 cita rewriter como hook customizado preexistente; nenhum gerador de rewriter/guardrails foi encontrado no repositório. Testes de preservação de hooks customizados passaram. Não corrigir a geração do trackfw para este defeito. Guards trackfw emitem deny corretamente.
