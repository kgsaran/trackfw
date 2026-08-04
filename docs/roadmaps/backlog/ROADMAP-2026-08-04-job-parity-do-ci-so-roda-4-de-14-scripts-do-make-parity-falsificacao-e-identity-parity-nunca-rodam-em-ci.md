---
status: backlog
date: 2026-08-04
req: "docs/req/REQ-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md"
squad: "ares-tf"
---

# Roadmap: job parity do CI so roda 4 de 14 scripts do make parity (falsificacao e identity-parity nunca rodam em CI)

> Created: 2026-08-04 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: `docs/req/REQ-2026-08-04-job-parity-do-ci-so-roda-4-de-14-scripts-do-make-parity-falsificacao-e-identity-parity-nunca-rodam-em-ci.md`

10 dos 14 scripts de `make parity` (incluindo os 100 cenários de falsificação P4 de
`check-gates-falsify.sh` e `check-identity-parity.sh`) nunca rodam em CI — só se alguém lembrar de
`make quality` local. `.github/workflows/quality.yml`'s job `parity` lista 4 scripts manualmente em
vez de chamar `make parity`, então todo gate novo nasce fora do CI por padrão.

**Dependência bloqueante explícita**: `check-identity-parity.sh` está vermelho no `main` agora
(REQ-2026-08-04-json-marshalindent-do-go-escapa-html...). Ligar esse script no CI **antes** desse
bug ser corrigido deixaria o CI permanentemente vermelho — a ordem de merge importa: a REQ do
HTML-escaping precisa fechar primeiro.

## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ] Job `parity` roda `make parity` (ou equivalente sincronizado) em vez de listar scripts
- [ ] Tempo de execução medido no ambiente real do GitHub Actions, decisão de job separado/paralelo
      tomada com base na medição, não em suposição
- [ ] `release.yml` reavaliado e a decisão documentada
- [ ] Nenhum gate liga vermelho — REQ do HTML-escaping fechada antes desta Wave 1

## Wave 1 — Ligar make parity completo ao CI
> Dependencies: REQ-2026-08-04-json-marshalindent-do-go-escapa-html-e-diverge-de-node-python-...
> precisa estar Done antes de iniciar (bloqueante — ver Context)

### ML-1A — Trocar a lista manual de scripts pelo `make parity` no job `parity`
**Status:** pending
**Files affected:**
- `.github/workflows/quality.yml` (job `parity`)
**Actions:**
1. Confirmar que a REQ-2026-08-04-json-marshalindent-do-go-escapa-html... está `Done` antes de
   começar — se não estiver, este ML fica bloqueado (não iniciar antes).
2. Trocar as 4 linhas `- run: scripts/check-*.sh` do job `parity` por `- run: make parity` (o job já
   tem Go/Node/npm/Python configurados; confirmar que `make parity` não precisa de nenhuma etapa de
   setup adicional além do que o job já faz, ex: `npm ci`, `pip install pypi/`).
3. Rodar o workflow (via PR de teste ou `act`/execução manual) e medir o tempo total do job `parity`
   completo, especialmente a contribuição de `check-gates-falsify.sh`.
4. Se o tempo total for razoável para feedback de PR (julgamento do implementador com base na
   medição — não há número mágico pré-definido nesta REQ, ver P1 em
   `docs/gate-design-principles.md`), manter sequencial. Se for excessivo, separar
   `check-gates-falsify.sh` (e talvez `check-identity-parity.sh`, que também é pesado) num job
   paralelo próprio, documentando a decisão e o tempo medido que a motivou.
**Acceptance criteria:**
- [ ] Workflow roda verde de ponta a ponta com `make parity` completo
- [ ] Tempo de execução registrado nesta seção do roadmap
- [ ] `trackfw validate` sem violações

### ML-1B — Reavaliar release.yml
**Status:** pending
**Files affected:**
- `.github/workflows/release.yml`
**Actions:** Decidir e documentar (comentário no `.yml` ou nesta seção do roadmap) se o subconjunto
reduzido de 3 scripts em `release.yml` é intencional (ex: release só roda após `quality.yml` já ter
passado no mesmo commit, então redundância completa seria desperdício) ou se precisa do mesmo
tratamento do ML-1A.
**Acceptance criteria:**
- [ ] Decisão registrada, com justificativa
- [ ] Se decidido expandir, `release.yml` atualizado e testado (worfklow_dispatch manual ou
      equivalente, sem precisar cortar uma tag real só para testar)
