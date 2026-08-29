---
status: done
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-req-move-no-cli-node-nao-trata-erros-stack-trace-nao-capturado-em-vez-de-mensagem-limpa.md"
squad: ""
---

# Roadmap: req move no Node — try/catch local no comando, sem handler global

> Created: 2026-08-04 | Status: done

REQ: REQ-2026-08-04-req-move-no-cli-node-nao-trata-erros-stack-trace-nao-capturado-em-vez-de-mensagem-limpa.md

## Diagnóstico / Contexto

`npm/src/commands/req.js`, subcomando `move <name> <status>` (linha ~70-74), chama `moveREQ(name,
status)` dentro de `.action(async (...) => {...})` sem `try/catch`. Erros lançados por
`moveREQ`/`findREQ` sobem como rejeição de Promise não tratada — Node imprime stack trace bruto em vez
de mensagem limpa, divergindo de Go (`cobra` formata `RunE` error como `Error: <msg>`) e Python
(`_cmd_move` já tem `try/except RuntimeError`). Decisão de escopo em
`docs/adr/ADR-2026-08-04-req-move-no-node-try-catch-local-no-comando-sem-handler-global.md`: corrigir
localmente em `req.js` (não introduzir handler global nem corrigir `roadmap.js`, que tem o mesmo padrão
mas fica para uma REQ própria).

## Acceptance Criteria

- [x] AC1 — `trackfw req move` (Node.js) imprime `Error: <mensagem>` em stderr e sai com código não-zero
      para todas as condições de erro hoje lançadas
- [x] AC2 — Nenhum stack trace JavaScript na saída para esses erros
- [x] AC3 — `trackfw req list` auditado quanto a caminhos de erro equivalentes
- [x] AC4 — Testes de regressão cobrindo pelo menos um caso de erro de `req move`, checando stderr e
      exit code

## Wave 1 — Fix local no comando (1 ML)
> Dependências: Independente

### ML-1A — try/catch em `req.js`
**Status:** ✅ Concluído
**Arquivos afetados:**
- `npm/src/commands/req.js` (subcomando `move`, linha ~70-74; auditar `list`, linha ~64-67)
- Teste novo em `npm/tests/` cobrindo o caminho de erro

**Ações:**
1. Envolver a chamada a `moveREQ(name, status)` em `.action(async (name, status) => {...})` com
   `try/catch`: no `catch`, imprimir `console.error(\`Error: ${err.message}\`)` e definir
   `process.exitCode = 1` (não usar `process.exit(1)` direto dentro do catch se houver I/O pendente —
   preferir `process.exitCode` para permitir flush de stdout/stderr antes de sair).
2. Auditar `listREQs` quanto a caminhos de erro que hoje não lançam (`try/catch` interno já engole erros
   de leitura de diretório) — se não houver caminho de erro relevante, documentar essa constatação no
   commit, sem alterar `list`.
3. Teste novo: `req move <nome-inexistente> done` via CLI (spawn do processo Node ou chamada direta ao
   handler, conforme padrão já usado em `npm/tests/req_move.test.js`) — assert que stderr contém
   `Error:` e nenhum stack trace, e que o processo sai com código não-zero.

**Critérios de aceite:**
- [x] `npm --prefix npm test` verde, incluindo o teste novo
- [x] `trackfw req move <nome-inexistente> done` (via `node npm/bin/trackfw`) imprime `Error: REQ "..."
      not found...` em stderr, sem stack trace, exit code != 0

**Comandos de validação:** `npm --prefix npm test`
