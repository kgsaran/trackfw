---
status: Open
date: 2026-08-04
author: "Zeus"
adr: "docs/adr/ADR-2026-08-04-req-move-no-node-try-catch-local-no-comando-sem-handler-global.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-node-try-catch-local.md"
---

# REQ: req move no CLI Node não trata erros — stack trace não capturado em vez de mensagem limpa

> Date: 2026-08-04 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Achado durante a auditoria de conformidade da Wave 2 do roadmap
`ROADMAP-2026-08-04-req-move-list-subpastas-e-move-fisico.md` (agente ML-2A, prova de paridade): ao rodar
`req move REQ-state.md status-invalido-xyz` nos 3 binários para confirmar rejeição de status inválido, o
CLI Go e o CLI Python imprimem uma mensagem de erro limpa em stderr e saem com código não-zero; o CLI
Node.js despeja um **stack trace JavaScript não capturado** no terminal em vez de uma mensagem de erro
utilizável.

Causa raiz: `npm/src/commands/req.js` (subcomando `move <name> <status>`, linha ~70-74) chama
`moveREQ(name, status)` dentro de um `.action(async (name, status) => { moveREQ(name, status) })` **sem
`try/catch`**. Como `moveREQ`/`findREQ` lançam (`throw new Error(...)`) em qualquer condição de erro
(REQ não encontrada, status vazio, sem frontmatter, e agora também status inválido no move físico — ver
`ADR-2026-08-04-req-move-list-reusar-roadmap-namespacing-para-req-e-mover-fisicamente-o-arquivo.md`), o
erro sobe como rejeição não tratada de uma Promise `async`, e `npm/bin/trackfw`
(`require('../src/commands/index').createProgram().parseAsync(process.argv)`) não tem nenhum handler
global de erro (`process.on('unhandledRejection', ...)` ou equivalente) para convertê-la numa saída
limpa.

Comparativamente:
- **Go**: `internal/commands/req.go`, `RunE: func(cmd *cobra.Command, args []string) error {...}` —
  cobra formata qualquer erro retornado como `Error: <mensagem>` em stderr e sai com código 1.
- **Python**: `pypi/trackfw/commands/req.py`, `_cmd_move` já tem `try/except RuntimeError as exc: print(f"Error:
  {exc}", file=sys.stderr); sys.exit(1)`.
- **Node.js**: sem equivalente — nem no nível do comando (`req.js`) nem globalmente (`npm/bin/trackfw`).

**Escopo mais amplo constatado, não coberto por esta REQ:** o mesmo padrão (`.action(async (...) => {
fnQueLança(...) })` sem `try/catch`) também existe em `npm/src/commands/roadmap.js` (`move <name>
<state>`, linha ~37-41) e possivelmente em outros comandos do CLI Node — não foi feito levantamento
exaustivo. Esta REQ cobre apenas `req move`/`req list`/`req new`; um levantamento e correção sistêmicos
de todos os comandos (ou um handler global único em `npm/bin/trackfw`) ficam de fora do escopo aqui —
ver "Negative Scope".

### Por que importa

- Stack trace bruto expõe caminhos internos do módulo e não comunica a causa do erro de forma acionável
  para quem está usando o CLI — pior experiência que os outros dois CLIs para o mesmo erro.
- Scripts/CI que invocam `trackfw req move` e fazem parsing de stderr para decidir o próximo passo
  (ex.: hooks descritos em `CLAUDE.md` §"Sinalização de Atenção") não podem confiar num formato estável
  de mensagem de erro no Node.

## Acceptance Criteria
- [ ] AC1 — `trackfw req move` (Node.js) imprime uma mensagem de erro limpa em stderr (formato
      equivalente a `Error: <mensagem>`, mesmo texto de erro já lançado por `moveREQ`/`findREQ`) e sai
      com código de saída não-zero, para todas as condições de erro hoje lançadas (`status is required`,
      `REQ "..." not found`, "has no frontmatter status/header...", `invalid state "..."`).
  - [ ] AC2 — Nenhum stack trace JavaScript aparece na saída padrão do usuário para esses erros.
- [ ] AC3 — `trackfw req list` recebe o mesmo tratamento se algum caminho de erro relevante existir
      (auditar; hoje `listREQs` não lança erros conhecidos, mas confirmar).
  - [ ] AC4 — Solução preferencialmente local ao comando `req` (`.action` com `try/catch` explícito,
      mesmo padrão que outros comandos do Node CLI já usam quando tratam erros — verificar se existe um
      helper compartilhado antes de duplicar lógica) — **não** expandir para um handler global em
      `npm/bin/trackfw` nem para `roadmap.js` nesta REQ (ver Negative Scope).
- [ ] AC5 — Testes de regressão cobrindo pelo menos um caso de erro de `req move` verificando stderr e
      exit code, sem depender de capturar stack trace.

## Negative Scope — o que esta REQ NÃO faz

- **Não corrige `roadmap.js`** nem varre os demais comandos do CLI Node em busca do mesmo padrão —
  constatado como problema mais amplo durante o levantamento, mas fora do escopo aqui. Se o padrão se
  confirmar generalizado, é candidato a uma REQ própria (ou um handler de erro global único em
  `npm/bin/trackfw`, decisão de design que merece ADR).
- **Não altera o formato de mensagem de erro em si** (texto das mensagens lançadas por `moveREQ`/
  `findREQ`) — só a forma como o CLI captura e apresenta esses erros.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: `docs/adr/ADR-2026-08-04-req-move-no-node-try-catch-local-no-comando-sem-handler-global.md`

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/wip/ROADMAP-2026-08-04-req-move-node-try-catch-local.md`
