---
status: Proposed
date: 2026-08-04
author: "Zeus"
---

# ADR: req move no Node — try/catch local no comando, sem handler global

> Date: 2026-08-04 | Status: Proposed

## Context

`REQ-2026-08-04-req-move-no-cli-node-nao-trata-erros-stack-trace-nao-capturado-em-vez-de-mensagem-limpa.md`
documenta que `npm/src/commands/req.js`, no subcomando `move <name> <status>`, chama `moveREQ(name,
status)` dentro de `.action(async (...) => {...})` sem `try/catch`. Qualquer erro lançado por
`moveREQ`/`findREQ` (`throw new Error(...)`) sobe como rejeição de Promise não tratada — Node imprime um
stack trace bruto em vez da mensagem de erro limpa que Go (`cobra` formatando `RunE` error) e Python
(`try/except RuntimeError` em `_cmd_move`) já produzem para o mesmo erro.

Durante o levantamento, constatou-se que o mesmo padrão (`.action` sem `try/catch`) também existe em
`npm/src/commands/roadmap.js` (`move <name> <state>`) e possivelmente em outros comandos — não há
handler global de erro em `npm/bin/trackfw` (`process.on('unhandledRejection', ...)` ou equivalente).

## Decision

A correção fica **local ao comando `req`** — `try/catch` explícito em volta de `moveREQ`/`listREQs`
dentro de `npm/src/commands/req.js`, formatando o erro capturado como `Error: <mensagem>` em stderr e
saindo com código não-zero (`process.exitCode = 1` ou `process.exit(1)`), espelhando o formato que Go e
Python já produzem para o mesmo erro.

Não se estende a correção a `roadmap.js` nem se introduz um handler global em `npm/bin/trackfw` nesta
REQ — ver Alternativas Consideradas.

## Consequences

**Positivas:**
- `trackfw req move` no Node passa a ter paridade de experiência de erro com Go/Python para os casos já
  cobertos por esta REQ, sem esperar por uma reformulação maior do tratamento de erros do CLI Node.
- Mudança pequena e isolada, de baixo risco, fácil de revisar e testar.

**Negativas:**
- O mesmo defeito continua presente em `roadmap.js` e potencialmente outros comandos — usuários ainda
  verão stack traces em outros caminhos de erro do CLI Node até uma correção sistêmica futura.
- Se a correção sistêmica (handler global) vier depois, o `try/catch` local em `req.js` se torna
  redundante (mas inofensivo — um handler global normalmente só captura o que escapou de handlers mais
  específicos).

## Alternatives Considered

- **Handler global único em `npm/bin/trackfw`** (`process.on('unhandledRejection', ...)` ou usar a API
  de erro do próprio `commander`). Rejeitada nesta REQ por ampliar o escopo além do problema relatado
  (`req move`) para todos os comandos do CLI Node — decisão maior, que impacta comportamento de todo o
  CLI, merece sua própria REQ com levantamento completo de quais comandos hoje dependem (ainda que
  acidentalmente) de deixar o stack trace vazar, e testes de regressão para cada um.
- **Não corrigir agora, registrar só como débito técnico.** Rejeitada: o fix local é pequeno, de baixo
  risco, e a REQ já foi aberta — vale corrigir no escopo em que foi encontrado em vez de adiar
  indefinidamente.
