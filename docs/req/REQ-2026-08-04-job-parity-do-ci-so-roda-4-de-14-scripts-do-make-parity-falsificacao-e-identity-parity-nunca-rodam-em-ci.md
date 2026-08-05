---
status: Done
date: 2026-08-04
author: "kg.saran@gmail.com"
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md"
---

# REQ: job parity do CI so roda 4 de 14 scripts do make parity (falsificacao e identity-parity nunca rodam em CI)

> Date: 2026-08-04 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->
Achado durante a auditoria da REQ-2026-08-04-json-marshalindent... (o bug de HTML-escaping só
apareceu porque `check-identity-parity.sh` foi rodado manualmente via `make quality` — nunca teria
sido pego só olhando os checks do GitHub Actions).

Comparação direta, `Makefile` (alvo `parity`) vs `.github/workflows/*.yml` (`grep -n "check-.*\.sh"`
em todos os workflows):

| Script em `make parity` | Roda em algum job de CI? |
|---|---|
| `check-cli-parity.sh` | ✅ `quality.yml` job `parity` |
| `check-validate-parity.sh` | ✅ `quality.yml` job `parity` |
| `check-static-assets.sh` | ✅ `quality.yml` jobs `go` e `parity` |
| `check-integration-assets.sh` | ✅ `quality.yml` jobs `go` e `parity` |
| `check-referential-integrity.sh` | ❌ nunca |
| `check-identity-parity.sh` | ❌ nunca |
| `check-artifact-parity.sh` | ❌ nunca |
| `check-barrier.sh` | ❌ nunca |
| `check-slash-parity.sh` | ❌ nunca |
| `check-rules-parity.sh` | ❌ nunca |
| `check-update-parity.sh` | ❌ nunca |
| `check-roadmap-move-parity.sh` | ❌ nunca |
| `check-branch-new-parity.sh` | ❌ nunca (novo, adicionado nesta sessão — já nasceu fora do CI) |
| `check-gates-falsify.sh` | ❌ nunca — **a suíte inteira de 100 cenários de falsificação (P4),
  cujo propósito explícito é provar que os outros gates não são vazios (`docs/gate-design-principles.md`),
  nunca roda de forma automatizada** |

10 dos 14 scripts de `make parity` — incluindo toda a prova de falsificabilidade P4 — só rodam se
alguém lembrar de executar `make quality` localmente antes de abrir/mergear um PR. `release.yml`
roda um subconjunto ainda menor (3 scripts). Nenhum workflow chama `make parity` diretamente; cada
um lista os scripts manualmente, o que significa que **todo gate novo nasce fora do CI por padrão**,
a menos que alguém lembre de adicionar a linha correspondente no `.yml` — exatamente o que aconteceu
com `check-branch-new-parity.sh` nesta mesma sessão.

## Acceptance Criteria
- [x] O job `parity` de `.github/workflows/quality.yml` roda `make parity` diretamente (ou uma
      lista que se mantém sincronizada automaticamente com o `Makefile` — preferir `make parity`
      pela simplicidade e por eliminar o risco de re-drift) em vez de listar os scripts manualmente
- [x] Medir o tempo real de execução de `make parity` completo (incluindo os 100 cenários de
      `check-gates-falsify.sh`) no ambiente do GitHub Actions (`ubuntu-latest`) — se ultrapassar um
      limite razoável para feedback de PR (definir o limite nesta REQ ou no ML, com base na medição
      real, não em suposição), avaliar rodar `check-gates-falsify.sh` como job separado em paralelo
      aos demais, em vez de sequencial dentro do mesmo job — mas só se a medição justificar,
      não preventivamente
- [x] `release.yml` também reavaliado: decidir explicitamente se o subconjunto reduzido de scripts
      ali é intencional (release já depende do `quality.yml` ter passado antes?) ou se também precisa
      de `make parity` completo — documentar a decisão, não deixar implícito
- [x] Nenhum gate existente (incluindo os 10 hoje fora do CI) pode ficar permanentemente vermelho
      quando ligado ao CI pela primeira vez — se algum estiver quebrado no estado atual do `main`
      (ex: o bug de HTML-escaping da REQ irmã), a ordem de merge importa: corrigir o gate primeiro,
      ligar o CI depois, não o contrário
- [x] `trackfw validate` sem violações

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: N/A — correção de configuração de CI, sem decisão arquitetural nova.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md`
