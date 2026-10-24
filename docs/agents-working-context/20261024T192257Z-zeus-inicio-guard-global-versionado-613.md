# zeus — início: o update não pode depender do guard global num arquivo versionado (#613)

- **Começou:** o KG autorizou seguir com a #613. Pela Regra Dura, a correção entra como ML novo na REQ do #599 (REQ-2026-10-23, guard por gerações), cujo roadmap volta para `wip`.
- **Decidido (ADR-2026-10-04, D15):** o gerador de projeto deixa de consultar o guard global. A fiação de projeto passa a ser sempre emitida. Isso substitui a D14 e a decisão 4 do ADR-2026-08-06. O custo de rodar o guard duas vezes, medido: `credential` com mediana de 12,5 ms e `git-branch` com 10,6 ms por chamada.
- **Plano:**
  - Wave 4: threat model e enumeração de sítios (hades-tf).
  - Wave 5: implementação (apolo-tf).
  - Wave 6: revisões (hefesto-tf e hades-tf).
  - Wave 7: fechamento.
