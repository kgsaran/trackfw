---
status: backlog
date: 2026-07-20
req: "docs/requisições/afrodite/backlog/REQ-2026-06-19-architect-command-guidelines-ML-1B.md"
---

# Roadmap ML-1B — Node.js: architect.md + regras de arquitetura no rules block

## Visão Geral
Adicionar o slash command `/trackfw:architect` (`architect.md`) e injetar as `Architecture Directives` no gerador Node.js.

## Tasks
- [ ] Adicionar `architect.md` em `generateClaudeCommands` e `generateClaudeCommandsForce` em `npm/src/generators/init.js`
- [ ] Atualizar `trackfwRulesBlock()` em `npm/src/generators/init.js` com a seção `### Architecture Directives (mandatory)`
- [ ] Criar/atualizar testes em `npm/tests/generators.test.js`
- [ ] Rodar os testes `node --test npm/tests/generators.test.js` e garantir aprovação
- [ ] Atualizar o roadmap raiz `docs/roadmaps/ROADMAP-2026-06-19-architect-command-guidelines.md`
