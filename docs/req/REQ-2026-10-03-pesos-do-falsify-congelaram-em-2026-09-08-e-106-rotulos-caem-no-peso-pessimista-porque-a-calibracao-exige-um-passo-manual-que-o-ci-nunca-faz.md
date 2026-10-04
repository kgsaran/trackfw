---
status: Open
date: 2026-10-03
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-03-pesos-do-falsify-se-recalibram-a-partir-do-ci-que-grava-o-arquivo-de-tempos-em-todo-run.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md"
---

# REQ: pesos do falsify congelaram em 2026-09-08 e 106 rotulos caem no peso pessimista porque a calibracao exige um passo manual que o CI nunca faz

> Date: 2026-10-03 | Status: Open
| GitHub Issue: #403

## Motivation

Os pesos do balanceador do falsify foram calibrados uma vez (#295, 2026-09-08). De lá para cá, 106 rótulos
entraram sem peso e herdam o máximo calibrado (54 s, cerca de 105× a mediana). Causa: recalibrar exige
setar `FALSIFY_TIMING_FILE` e seguir uma receita manual, e nenhum job faz isso. No CI o efeito em tempo de
parede é nulo hoje, porque o caminho crítico é o `parity-other-gates`. O efeito aparece no `make quality`
local e num número que ninguém pode auditar. Decisão no ADR ligado.

## Acceptance Criteria

- [ ] **AC1** — 🔴 **Wave 0:** threat model, com parecer em `docs/seguranca/`. Cobre: a instrumentação
  ligada no CI muda algum veredito ou contagem de rótulos do shard? `gh run download` de um run de fork
  ou de outro repositório? Um arquivo de pesos adulterado consegue tirar um cenário da cobertura (a
  guarda `check-falsify-shard-coverage.sh` pega isso)?
- [ ] **AC2** — O job `parity-falsify-shard` grava o arquivo de marcas dentro de `falsify-shard-out/`, e
  o artefato do run o contém, um por shard. Medido num run real do PR.
- [ ] **AC3** — `make falsify-recalibrate RUN=<id>` refaz `scripts/falsify-scenario-weights.json` a partir
  desse run. Reprova (exit ≠ 0, com mensagem) se faltar o arquivo de marcas de algum shard, ou se o run não
  for do repositório upstream.
- [ ] **AC4** — Recalibrado a partir do run do PR: rótulos sem peso calibrado = 0, ou o resto nomeado e
  explicado.
- [ ] **AC5** — O `gen-falsify-chunks.py` imprime uma linha de resumo `N de M rótulos sem peso calibrado
  (X%)`, sem mudar o exit code.
- [ ] **AC6** — Os 4 shards continuam verdes no CI, e a guarda de cobertura dos shards continua verde.
- [ ] **AC7** — Medição de volta: a distribuição de linhas e rótulos por chunk, antes e depois da
  recalibração, com o desvio máximo/mínimo. Se o desequilíbrio não cair, isso é registrado como medido, e
  não omitido.
- [ ] **AC8** — Cada teste ou gate novo declara o que afirma e reprova sem a correção.
- [ ] **AC9** — `make quality` EXIT=0 com a máquina ociosa; CI verde.

## Negative scope

- **Mudar o fallback** (mediana ou chave de dados): fora (ADR, alternativas).
- **Gate com teto de fração:** fora (decisão do KG).
- **Mudar o número de shards** ou o algoritmo de empacotamento (LPT).
- **Recalibração automática** num job agendado.

## Linked ADR
ADR: docs/adr/ADR-2026-10-03-pesos-do-falsify-se-recalibram-a-partir-do-ci-que-grava-o-arquivo-de-tempos-em-todo-run.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md
