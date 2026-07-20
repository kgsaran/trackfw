---
status: wip
date: 2026-07-20
req: "docs/requisições/afrodite/wip/REQ-2026-07-19-global-adrs-governance-ML-2B.md"
---

# Roadmap: ML-2B — Node.js: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan

## Status: 🔄 WIP
**Agente:** Afrodite (Frontend/Node Specialist)

## Ações ML-2B Node.js:
1. Em `npm/src/config/index.js`, adicionar `strict_ci_paths` no `defaults()` (default `false`) e no parse/normalização de YAML.
2. Em `npm/src/validator/index.js`:
   - Tratar `adr_dirs` inexistentes: adicionar em `warnings` se `strict_ci_paths: false` (default) ou em `violations` se `true`.
   - Na regra `adr_orphan`, isentar arquivos fora da raiz do projeto local (`cwd`).
3. Adicionar testes unitários em `npm/tests/validator.test.js`.
4. Rodar `node --test` e validar todos os testes verdes.
