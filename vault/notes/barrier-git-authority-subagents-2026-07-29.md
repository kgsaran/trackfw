# Barrier e autoridade Git dos agentes — 2026-07-29

## Contexto

O harness anterior descrevia o protocolo de conclusão dos subagents com commit e push. Isso é
incompatível com o fluxo operacional adotado para evitar sobrescrita, remoção ou concorrência de
trabalho entre agentes.

## Decisão

O papel canônico `trackfw_architect` é a única autoridade Git. Agentes especialistas implementam,
validam e reportam por handoff, mas não executam `checkout`, `branch`, `commit`, `push`, `merge`,
`rebase` ou operações destrutivas. O nome personalizado exibido ao usuário, como `zeus-tf`, não
altera essa regra.

O barrier deve auditar o diff e os resultados dos especialistas antes de o orquestrador consolidar
e commitar as alterações.

## Referências

- `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
- `docs/req/REQ-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`
- `docs/roadmaps/done/ROADMAP-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

