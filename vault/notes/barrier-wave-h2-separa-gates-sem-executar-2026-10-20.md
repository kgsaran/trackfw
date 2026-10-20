# Um heading H2 encerra a wave; gates depois dele ficam fora da barreira

Data: 2026-10-20 · Zeus · conclusão do ML-1H da REQ-2026-09-05.

## Evidência

O roadmap tinha `## Wave 2`, ML-1H e, antes de `**Gates da wave:**`, dois títulos H2 de contexto: `## Triagem pós-v8` e `## Plano executável`. O primeiro `barrier --wave 2 --json --trust-local-gates` retornou `status=passed`, porém o check `gates` continha `commands: []` e o ML-2C posterior também não aparecia em `mls_complete`.

Isso não provava os comandos que o arquiteto pretendia executar. O `make quality` independente já havia passado, mas não corrigia a ligação estrutural dos gates à wave.

## Causa e correção

`internal/roadmapdoc/roadmapdoc.go:495` encerra o bloco da wave no próximo `## `, inclusive títulos que não são waves. É o contrato existente, também exercitado por `parsewaves_fence_test.go`; não foi alterado nesta frente.

Os dois títulos de contexto viraram texto em negrito, mantendo ML-2C e os quatro comandos dentro da Wave 2. A auditoria deve conferir o array `commands`, os MLs enumerados e o exit de cada comando no JSON; um check `gates: passed` com lista vazia só prova que não havia gate associado ao bloco.

## Regra prática

Dentro de uma wave, use prosa ou headings subordinados para contexto, sem inserir outro H2 antes dos MLs/gates que pertencem a ela. Ao encerrar uma wave, valide a lista extraída contra a lista planejada; não deduza execução apenas de uma marca verde. Não transformar este achado de autoria do roadmap em mudança de parser sem uma REQ própria.
