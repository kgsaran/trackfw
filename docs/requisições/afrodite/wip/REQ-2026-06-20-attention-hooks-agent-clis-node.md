---
status: Open
date: 2026-07-20
author: Afrodite
roadmap: "docs/roadmap/afrodite/wip/ROADMAP-2026-06-20-attention-hooks-agent-clis-node.md"
---

# REQ: Injetores de Hooks de Atenção para os 7 CLIs no CLI Node.js (ML-2A a ML-2G)

## Motivation
Implementar os injetores de hooks de atenção para os 7 CLIs suportados no CLI Node.js (`npm/src/`), garantindo paridade e injeção idempotente em `trackfw init` e `discover --init`, além de testes unitários verdes.

## Acceptance Criteria
- [ ] Claude Code: `.claude/settings.json`
- [ ] Codex CLI: `.codex/hooks.json`
- [ ] Gemini CLI: `.gemini/settings.json`
- [ ] Kiro: `.kiro/hooks/trackfw-attention.json`
- [ ] GitHub Copilot: `.github/hooks/trackfw-attention.json`
- [ ] Cursor: `.cursor/hooks.json`
- [ ] Windsurf: instrução em `.windsurfrules`
- [ ] Integração no `trackfw init` e `discover --init`
- [ ] Testes unitários em `npm/tests/generators.test.js` passando com `node --test npm/tests/`
