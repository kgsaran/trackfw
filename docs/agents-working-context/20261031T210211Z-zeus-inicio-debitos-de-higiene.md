# zeus-tf — início dos débitos de higiene

- **Quando:** 2026-10-31.
- **Frente:** REQ-2026-08-30 reescrita a partir da medição na `main` 11.2.3: `release tag` confere o `package-lock` (AC1); `InstallGates` com bit de execução (AC2); aviso de `agent_models` e relatório do `update harness` (AC3); documentação do #633 e notas de vault (AC4). Item Python cancelado (v8); `pypi/build` não existe.
- **Decisão do KG:** desligar o pedido de credencial só sem terminal interativo. Medido: sem terminal o git já falha em 0 s (`Device not configured`), então não há mudança de código; vira nota de vault.
