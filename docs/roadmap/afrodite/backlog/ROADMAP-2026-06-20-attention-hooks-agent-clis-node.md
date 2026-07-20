---
status: wip
date: 2026-07-20
req: "docs/requisições/afrodite/wip/REQ-2026-06-20-attention-hooks-agent-clis-node.md"
---

# Roadmap: Injetores de Hooks de Atenção no CLI Node.js

> Criado em: 2026-07-20 | Status: 🔄 WIP

## Acceptance Criteria

- [ ] Injetores de hooks implementados para 7 CLIs no Node.js (`npm/src/generators/hooks.js` / `init.js`)
- [ ] Merge idempotente sem sobrescrever hooks pré-existentes
- [ ] Invocação dos injetores ao executar `trackfw init` e `discover --init`
- [ ] Testes unitários em `npm/tests/generators.test.js` passando com `node --test npm/tests/`

---

## Microlotes

### ML-2A a ML-2G (Node.js)
- [ ] Implementar funções em Node.js (`npm/src/generators/hooks.js` ou `npm/src/generators/init.js`)
- [ ] Atualizar `npm/src/commands/discover.js` e `npm/src/generators/init.js`
- [ ] Adicionar testes em `npm/tests/generators.test.js`
- [ ] Garantir `node --test npm/tests/` 100% verde
