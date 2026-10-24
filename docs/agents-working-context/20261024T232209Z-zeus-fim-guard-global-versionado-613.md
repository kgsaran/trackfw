# zeus — fim da implementação da #613 (D15), PR aberto (2026-10-24)

- **Mudou:**
  - **D15:** nenhum gerador de projeto nem o validador consulta o guard global. 15 sítios saíram e 16 funções foram removidas.
  - **AC9:** o teste por CLI, nos 7, roda `update` e depois `validate` e reprova em `origin/main`.
  - **ML-5B:** o `ml_scope` aceita o roadmap em qualquer estado.
  - **ML-6D:** a lista canônica de estados (`config.RoadmapStates()`) corrige o `discover` e o `context`, que omitiam `analyzing`.
  - **ML-7B:** os cenários 67 e 30 da falsificação foram reescritos.
  - **ML-7C/7D:** o git dos testes roda sem maintenance automático, que disputava o `RemoveAll` do `t.TempDir()`.
- **Medido:**
  - `make quality` rc=0 na 3ª rodada;
  - `bin/trackfw update` neste repositório, com o global instalado, não altera nenhum arquivo, e o `validate` dá rc=0.
- **Lições:**
  - a enumeração da Wave 4 cobriu só o Go e deixou de fora `scripts/`;
  - um `ls` com alias de ícone fez o hades rodar `init` dentro do repositório, o que ficou registrado na memória.
- **Pendente:** o CI do PR, o merge pelo KG, o encerramento da REQ e o release 11.0.1 (patch, aprovado pelo KG).
