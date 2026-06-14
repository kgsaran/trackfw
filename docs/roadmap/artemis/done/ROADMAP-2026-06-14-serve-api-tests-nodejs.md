# Roadmap: Testes Node.js — api_board, api_file, api_metrics (ML-4B)

> Criado em: 2026-06-14 | Status: WIP
> REQ: docs/requisições/artemis/wip/REQ-2026-06-14-serve-api-tests-nodejs.md

## Diagnóstico / Contexto
O roadmap feat/v2.7.0-trackfw-serve-ui (ML-4B) requer testes Node.js para as três APIs do `trackfw serve`:
- `api_board.js` — kanban em flat e by_agent mode
- `api_file.js` — leitura de arquivo com path traversal protection
- `api_metrics.js` — métricas de cycle time com e sem log

## Wave 1 — Implementar e validar (independente)

### ML-1A — Criar npm/tests/serve_api.test.js
**Status:** ✅ Concluído
**Arquivos afetados:** `npm/tests/serve_api.test.js` (novo)
**Ações:**
- 8 testes usando o padrão do projeto (test runner customizado sem Jest/Mocha)
- api_board: flat mode, by_agent mode, board vazio
- api_file: path válido 200, path traversal 403, path fora dos dirs 403
- api_metrics: sem log retorna zeros, com log calcula cycle_time_avg_days

**Critérios de aceite:**
- [x] `node npm/tests/serve_api.test.js` verde (8/8 passed)
- [x] Nenhum teste existente regride

**Comandos de validação:**
```bash
node npm/tests/serve_api.test.js
node npm/tests/validator.test.js
node npm/tests/baseline.test.js
```
