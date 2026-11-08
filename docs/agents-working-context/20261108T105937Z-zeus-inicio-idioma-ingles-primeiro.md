# Zeus — início da REQ de idioma do produto (inglês primeiro)

- **REQ:** `docs/req/REQ-2026-11-02-idioma-do-produto-ingles-primeiro-com-legenda-de-status-aceitando-ingles-e-portugues.md`, tirada do backlog por decisão do KG em 2026-11-08.
- **Achado na leitura dos parsers (antes da ADR):** o grupo 3 da REQ (contrato lido pelo binário) é maior que a legenda de status. Também só existem em português: `**Gates da wave:**` (`roadmapdoc.GatesHeaderRe`), `Caducou:` (`LapsedContinuationRe`) e as palavras de término `abandonado`/`cancelado` (`StatusCategory`). Já são bilíngues: `**Acceptance criteria|Critérios de aceite:**`, `**Files affected|Arquivos afetados:**`, `## Acceptance Criteria|Critérios de aceite` e o vocabulário de completo (`✅`, `done`, `concluido`).
- **Próximo:** ADR do contrato bilíngue → roadmap → wip → branch → Wave 0 com hades-tf (inventário do AC1).
