---
id: REQ-2026-06-19-architect-command-guidelines-ML-1B
title: Slash command /trackfw:architect + diretrizes de arquitetura (Node.js ML-1B)
status: Open
priority: high
type: feature
created: 2026-07-20
author: afrodite
---

# REQ: Slash command /trackfw:architect + diretrizes de arquitetura (Node.js ML-1B)

## Problema
Times não técnicos que usam o trackfw não têm orientação sobre stack e arquitetura. Os agentes tomam decisões técnicas arbitrárias. É necessário expor o slash command `/trackfw:architect` e injetar as `Architecture Directives` no gerador Node.js.

## Requisitos
1. Em `generateClaudeCommands()` e `generateClaudeCommandsForce()` em `npm/src/generators/init.js`, adicionar `'architect.md'` ao mapa `commands` com o conteúdo completo do slash command.
2. Em `trackfwRulesBlock()`, garantir a inclusão da seção `### Architecture Directives (mandatory)` com as 8 diretrizes obrigatórias.
3. Adicionar/atualizar testes unitários em `npm/tests/generators.test.js` verificando que `architect.md` é gerado após `generateClaudeCommands()` e que as diretrizes estão no bloco de regras.
4. Executar `node --test npm/tests/generators.test.js` garantindo 100% dos testes aprovados.
5. Marcar ML-1B como `✅ Concluído` em `docs/roadmaps/ROADMAP-2026-06-19-architect-command-guidelines.md`.
