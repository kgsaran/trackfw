# Zeus — higiene de governança (REQs com AC aberto e ADRs)

- Duas REQs de 2026-10-30 (`branch-new-e-branch-prune-…` e `teste-do-involucro-de-atencao-…`) estavam `Done` com todos os ACs desmarcados.
  - Conferi a entrega: os testes citados existem e passam por nome, e o AC4 do invólucro foi medido na VM, conforme o roadmap.
  - Foi só a marcação que faltou.
- `ADR-2026-09-26` passou para `Accepted`, com uma seção de aceite: a etapa 1 (aditiva) está em vigor e a etapa 2 (restritiva) não foi executada; ela exigiria REQ própria.
- `ADR-001` ganhou frontmatter.
- `trackfw validate`: 167 → 163 warnings; os 4 eliminados eram pontuais. Restam warnings históricos de grandfathering.
