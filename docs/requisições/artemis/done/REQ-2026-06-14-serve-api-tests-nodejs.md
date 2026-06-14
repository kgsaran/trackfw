# REQ-2026-06-14-serve-api-tests-nodejs

> Criada em: 2026-06-14 | Status: WIP

## Solicitante
KG via orquestrador Zeus (ML-4B do roadmap feat/v2.7.0-trackfw-serve-ui)

## Objetivo
Implementar testes Node.js para o `trackfw serve` — cobrindo `api_board`, `api_file` e `api_metrics`.

## Escopo
- `npm/tests/serve_api.test.js` (novo arquivo)
- Sem alterações nos arquivos de produção

## Critérios de Aceite
- [ ] `node npm/tests/serve_api.test.js` verde (8 testes)
- [ ] Nenhum teste existente regride
- [ ] Path traversal testado (403)
- [ ] Log ausente testado (zeros)
