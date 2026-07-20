---
status: wip
date: 2026-07-19
author: Afrodite
roadmap: "docs/roadmaps/ROADMAP-2026-07-19-global-adrs-governance.md"
---

# REQ: ML-1B - Suporte à Expansão de Til (~) no Node.js CLI (`adr_dirs`)

## Motivação
Permitir que o CLI Node.js do `trackfw` aceite caminhos com expansão de til (`~` ou `~/`) ao carregar `adr_dirs` nas configurações (`trackfw.yaml`) e durante a validação.

## Critérios de Aceite
1. Helper `expandPath(filePath)` em Node.js usando `os.homedir()` e `path.resolve`/`path.join`.
2. Aplicar expansão na leitura e validação de `adr_dirs` em `npm/src/config/` e `npm/src/validator/`.
3. Testes unitários em `npm/tests/config.test.js` e `npm/tests/validator.test.js` cobrindo o comportamento de expansão `~/...`.
4. Todos os testes `npm test` / `node --test` passando.
