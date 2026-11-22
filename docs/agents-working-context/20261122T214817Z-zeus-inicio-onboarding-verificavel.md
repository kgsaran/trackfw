# Zeus — início da REQ-2026-11-22: onboarding verificável

- **Agente:** zeus-tf (trackfw_architect)
- **Branch:** fix/onboarding-verificavel
- **Origem:** auditoria de onboarding da atena-tf (`docs/qualidade/2026-11-22-auditoria-onboarding.md`): 25 achados, 3 bloqueantes reproduzidos no binário 11.8.1.
- **Decisões do KG (2026-11-22):** uma REQ para o onboarding inteiro; site em inglês na raiz e pt-BR em `/pt/`.
- **Plano:** ADR-2026-11-22 D1–D6 (D2 pendente da Wave 0) → Wave 0 hades (varredura de flags do `init`, limites do brownfield, links) → Wave 1 apolo (`init`) ∥ afrodite (troca de idioma do site) → Wave 2 afrodite (conteúdo do site) ∥ atena (README) → Wave 3 apolo (teste pina README e site) → Wave 4 hefesto ∥ hades → make quality + CI.
