# Zeus — fim: memória de agente em subpasta (#646)

- **Entregue (branch `fix/memoria-de-agente-em-subpasta`):**
  - seção `## Memory location` só no destino Claude Code;
  - `## Dispatch cwd` no arquiteto;
  - regra `*/**/.claude/agent-memory/` no `.gitignore` via `init`/`update`, respeitando negação do consumidor que pode casar subpasta;
  - achado `stray-agent-memory` no `doctor`, com remédio citado e conteúdo ilegível nunca declarado vazio;
  - regra no `.gitignore` deste repositório.
- **Barriers:** waves 0 a 3 verdes com o binário da branch; as Waves 1 e 3 incluem `make quality` com exit 0.
- **Erro corrigido na auditoria:** a primeira correção do A-1 bloqueava também a negação ancorada na raiz (`!.claude/agent-memory/`); medi com `git check-ignore` e despachei o ajuste.
- **Pendente:** PR (quando o KG pedir); depois do merge, release 11.2.4 com #647 e #646 e o fechamento de governança no mesmo PR, por decisão do KG.
- **Aberto fora desta frente:** #649.
