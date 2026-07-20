---
status: done
date: 2026-07-20
agent: Afrodite
---

# REQ: ML-2B — Node.js: Bypass de CI/CD para Dirs Inexistentes + Isenção adr_orphan

## Descrição
Adicionar suporte a `strict_ci_paths` (default `false`) no CLI Node.js, tratar diretórios `adr_dirs` inexistentes como `Warning` (em vez de erro em `violations`), e isentar caminhos de ADR externos à raiz do projeto (`cwd`) da validação `adr_orphan`.

## Arquivos Afetados
- `npm/src/config/index.js`
- `npm/src/validator/index.js`
- `npm/tests/config.test.js`
- `npm/tests/validator.test.js`
