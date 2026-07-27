---
status: Done
date: 2026-07-20
author: afrodite
adr: "docs/adr/ADR-2026-07-19-global-adrs-governance.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-19-global-adrs-governance-ML-2B.md"
---

# REQ: ML-2B — Node.js: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan

> Date: 2026-07-20 | Status: Done

## Descrição
Adicionar suporte a `strict_ci_paths` (default `false`) no CLI Node.js, tratar diretórios `adr_dirs` inexistentes como `Warning` (em vez de erro em `violations`), e isentar caminhos de ADR externos à raiz do projeto (`cwd`) da validação `adr_orphan`.

## Arquivos Afetados
- `npm/src/config/index.js`
- `npm/src/validator/index.js`
- `npm/tests/config.test.js`
- `npm/tests/validator.test.js`

## Linked ADR

ADR: ADR-2026-07-19-global-adrs-governance.md

## Linked Roadmap

Roadmap: ROADMAP-2026-07-19-global-adrs-governance-ML-2B.md
