---
status: Open
date: 2026-10-08
author: ""
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md"
---

# REQ: push e ship avisam sobre branch de outro remote como trabalho nao mesclado

> Date: 2026-10-08 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Issue #547 (@lourivalgarciajunior, 2026-10-08, binário 9.3.3). Num repositório com mais de um remote (fork com
`origin` + `upstream`), `trackfw push` e `trackfw ship` avisam `branch "upstream/fix/..." appears to have unmerged
changes vs origin/main` — branch de OUTRO remote apresentada como trabalho nosso não mesclado.

Mecanismo (lido em `detectPendingSquashMerges`, `internal/commands/ship.go`): `git branch -r --no-merged origin/main`
lista refs de todos os remotes; `TrimPrefix(candidate, "origin/")` não altera `upstream/foo`; o sentinela vira
`origin/upstream/foo`, que não existe, e a decisão cai em "pendente". Nenhuma fixture de teste usa remote diferente
de `origin` — o caminho multi-remote não é exercitado (mesma família de "upstream não exercita o layout do consumidor").
Amplificador: `git fetch` sem poda acumula refs mortas (135 no fork do relator antes da poda).

**Decisão (saída 1 da issue):** recortar a população na origem — só candidatos do remote `origin`, de modo que o
`TrimPrefix` seja verdadeiro por construção.

**Escopo negativo:** não deriva o remote de `@{u}` (saída 2 da issue); não muda o predicado de "mesclada" nem a
consulta ao forge; não poda refs do usuário; não muda o caráter consultivo do aviso.

## Acceptance Criteria
- [ ] AC1 — Wave 0: enumeração de todo sítio que lista `branch -r` ou assume o prefixo `origin/` (push, ship, `branch prune`, validate, status) e decisão por sítio com medição
- [ ] AC2 — `push`/`ship` não avisam sobre branch de remote diferente de `origin`; branch de `origin` com trabalho pendente continua avisando (teste nas duas direções, fixture com dois remotes)
- [ ] AC3 — Mesmo defeito em outro sítio achado no AC1 → corrigido aqui com teste
- [ ] AC4 — Comentário na #547 com o resultado

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md
