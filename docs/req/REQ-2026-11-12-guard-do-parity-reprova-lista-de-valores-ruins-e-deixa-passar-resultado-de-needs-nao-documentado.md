---
status: Open
date: 2026-11-12
author: "Zeus (trackfw_architect)"
adr: "docs/adr/ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md"
---

# REQ: Guard do parity reprova lista de valores ruins e deixa passar resultado de needs nao documentado

> Date: 2026-11-12 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Discussion #664, comentário de 2026-11-06 (Lourival): num braço de matriz abandonado por falta de runner,
`needs.shim-byte-identity.result` chegou como `abandoned`, valor fora dos quatro documentados pelo GitHub. O
`shim-byte-identity-gate` reprovou, porque exige `success`. O primeiro passo do job `parity`
(`.github/workflows/quality.yml`, linha ~1010) testa a lista de valores ruins
(`contains(needs.*.result, 'failure' | 'cancelled' | 'skipped')`) e, com essa entrada, não dispara. Um agregador
com `if: always()` cuja guarda não dispara aprova sobre nada verificado, o que o próprio comentário do job
proíbe.

Conferido pelo arquiteto em `main` (`cdb8deac`): `contains(needs` aparece só em `quality.yml:1010`. Os scripts
de release (`verify-npm-channels.sh`, `verify-pypi-channel.py`) já exigem `success` quando a variável vem
preenchida. A Wave 0 refaz esse inventário com instrumento próprio (ADR D3).

## Acceptance Criteria
- [ ] **AC1 — Inventário medido.** A Wave 0 lista todo consumidor de `needs.*.result`, `contains(needs…)` e
  variável `*_RESULT` em `.github/workflows/` e `scripts/`, classificado como permitidos, proibidos ou
  diagnóstico, com contagem conferida por um segundo caminho. Todo sítio de proibidos que decide gate entra no
  ML-1A.
- [ ] **AC2 — `scripts/check-needs-success.sh`** (ADR D2): lê `NEEDS_JSON`, aprova só quando há pelo menos um job
  e todos têm `result == success`, e imprime o nome e o resultado de cada job reprovado. O `--self-test` passa,
  com braços nomeados: todos `success` (passa), `failure`, `cancelled`, `skipped`, `abandoned`, valor inventado,
  `{}`, string vazia e JSON inválido (reprovam). Também roda em `/bin/bash` 3.2.
- [ ] **AC3 — O `parity` usa o script.** O passo com `contains(...)` é substituído por uma chamada ao script com
  `NEEDS_JSON: ${{ toJSON(needs) }}`, como primeiro passo. Os outros sítios do AC1 recebem a mesma forma.
- [ ] **AC4 — Comentários coerentes.** O bloco de `quality.yml` (~948–955) que diz que a guarda do `parity` "testa
  apenas 'failure' e 'cancelled'" é reescrito para o estado novo.
- [ ] **AC5 — Self-test no `parity-rest`.** Uma linha no `Makefile` roda `scripts/check-needs-success.sh
  --self-test`. Contra-braço: reverter a lista de permitidos para a de proibidos faz o braço `abandoned`
  reprovar o self-test.
- [ ] **AC6 — Medição no CI.** O PR desta REQ mostra no log do `parity` a lista de jobs avaliados, todos
  `success`, e o check verde.
- [ ] **AC7 — `make quality` verde.**
- [ ] **AC8 — Resposta na Discussion #664**, com o link do PR.

## Negative scope

- Não muda a regra de dispensa por diff inerte do `shim-byte-identity-gate` (ADR-2026-11-06), nem a guarda dele,
  que já exige `success`.
- Não trata a falta de runner de macOS na conta do fork: a causa é externa ao repositório.
- Não muda `windows-census.yml` (`censo`/`apuracao`): é diagnóstico com `continue-on-error` e não decide merge.
  A Wave 0 confirma ou contesta.
- Não cria gate estático novo para a classe (ADR D4), salvo se a Wave 0 achar mais de um sítio.
- Não altera o conjunto R (required checks) nem o nome de nenhum check.

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: docs/adr/ADR-2026-11-12-agregador-de-ci-exige-success-dos-needs-em-vez-de-reprovar-lista-de-valores-ruins.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: docs/roadmaps/wip/ROADMAP-2026-11-12-guard-do-parity-reprova-lista-de-valores-ruins-e-deixa-passar-resultado-de-needs-nao-documentado.md
