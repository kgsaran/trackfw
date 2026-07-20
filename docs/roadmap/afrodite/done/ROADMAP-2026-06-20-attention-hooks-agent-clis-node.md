---
status: done
date: 2026-07-20
req: "docs/requisições/afrodite/done/REQ-2026-06-20-attention-hooks-agent-clis-node.md"
---

# Roadmap: Injetores de Hooks de Atenção no CLI Node.js

> Criado em: 2026-07-20 | Status: ✅ DONE

## Acceptance Criteria

- [x] Injetores de hooks implementados para 7 CLIs no Node.js (`npm/src/generators/hooks.js` / `init.js`)
- [x] Merge idempotente sem sobrescrever hooks pré-existentes
- [x] Invocação dos injetores ao executar `trackfw init` e `discover --init`
- [x] Testes unitários em `npm/tests/generators.test.js` cobrindo os 7 CLIs

---

## Microlotes

### ML-2A a ML-2G (Node.js)
- [x] Implementar funções em Node.js (`npm/src/generators/hooks.js` e `npm/src/generators/init.js`)
- [x] Atualizar `npm/src/commands/discover.js` e `npm/src/generators/init.js`
- [x] Adicionar testes em `npm/tests/generators.test.js`
- [x] Garantir paridade e resiliência dos testes unitários
