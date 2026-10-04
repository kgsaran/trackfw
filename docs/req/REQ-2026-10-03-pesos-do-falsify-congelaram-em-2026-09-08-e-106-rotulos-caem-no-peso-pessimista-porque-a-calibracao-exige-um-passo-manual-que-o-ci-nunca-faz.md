---
status: Done
date: 2026-10-03
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-03-pesos-do-falsify-se-recalibram-a-partir-do-ci-que-grava-o-arquivo-de-tempos-em-todo-run.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md"
---

# REQ: pesos do falsify congelaram em 2026-09-08 e 106 rotulos caem no peso pessimista porque a calibracao exige um passo manual que o CI nunca faz

> Date: 2026-10-03 | Status: Done
| GitHub Issue: #403

## Motivation

Os pesos do balanceador do falsify foram calibrados uma vez (#295, 2026-09-08). De lá para cá, 106 rótulos
entraram sem peso e herdam o máximo calibrado (54 s, cerca de 105× a mediana). Causa: recalibrar exige
setar `FALSIFY_TIMING_FILE` e seguir uma receita manual, e nenhum job faz isso. No CI o efeito em tempo de
parede é nulo hoje, porque o caminho crítico é o `parity-other-gates`. O efeito aparece no `make quality`
local e num número que ninguém pode auditar. Decisão no ADR ligado.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model, com parecer em `docs/seguranca/`. Cobre: a instrumentação
  ligada no CI muda algum veredito ou contagem de rótulos do shard? `gh run download` de um run de fork
  ou de outro repositório? Um arquivo de pesos adulterado consegue tirar um cenário da cobertura (a
  guarda `check-falsify-shard-coverage.sh` pega isso)?
      ✅ Evidência: `docs/seguranca/2026-10-03-wave0-recalibracao-pesos-falsify.md`: 8 ameaças, T1/T2 medidas (log do shard idêntico com e sem marcas), AJ-T3..T8 no ADR (D5)
- [x] **AC2** — O job `parity-falsify-shard` grava o arquivo de marcas dentro de `falsify-shard-out/`, e
  o artefato do run o contém, um por shard. Medido num run real do PR.
      ✅ Evidência: artefatos do run 37198827365: `falsify-shard-0..3/timing_0..3.log` com 26/22/24/22 linhas, conferido pelo arquiteto
- [x] **AC3** — `make falsify-recalibrate RUN=<id>` refaz `scripts/falsify-scenario-weights.json` a partir
  desse run. Reprova (exit ≠ 0, com mensagem) se faltar o arquivo de marcas de algum shard, ou se o run não
  for do repositório upstream.
      ✅ Evidência: `scripts/check-falsify-recalibrate.sh` em `parity-rest`: braços completo, shard faltando (checksum do destino preservado), fork (`head_repository`, sem `run download`) e `ts` malformado, mais 2 provas de mordida
- [x] **AC4** — Recalibrado a partir do run do PR: rótulos sem peso calibrado = 0, ou o resto nomeado e
  explicado.
      ✅ Evidência: `make falsify-recalibrate RUN=37198827365` rc=0; linha de resumo "0 de 204 rotulos sem peso calibrado (0.0%)"
- [x] **AC5** — O `gen-falsify-chunks.py` imprime uma linha de resumo `N de M rótulos sem peso calibrado
  (X%)`, sem mudar o exit code.
      ✅ Evidência: linha `N de M rotulos sem peso calibrado (X%)` em stderr; stdout (manifest) e exit inalterados
- [x] **AC6** — Os 4 shards continuam verdes no CI, e a guarda de cobertura dos shards continua verde.
      ✅ Evidência: CI do PR #517: 20/20, incluindo os 4 `parity-falsify-shard` e o `parity` (guarda de cobertura)
- [x] **AC7** — Medição de volta: a distribuição de linhas e rótulos por chunk, antes e depois da
  recalibração, com o desvio máximo/mínimo. Se o desequilíbrio não cair, isso é registrado como medido, e
  não omitido.
      ✅ Evidência: tabela antes/depois para N=4 e N=8 no roadmap (ML-2A); resíduo medido: 3 blocos sem rótulo com fallback único de 69 s (dois levam ~2 s) desequilibram N=8
- [x] **AC8** — Cada teste ou gate novo declara o que afirma e reprova sem a correção.
      ✅ Evidência: uma frase por braço no autoteste e prova de mordida para fork e shard faltando
- [x] **AC9** — `make quality` EXIT=0 com a máquina ociosa; CI verde.
      ✅ Evidência: `make quality` EXIT=0 (2930 linhas, falsify 347 OK / 0 FAIL); CI 20/20

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
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-03-pesos-do-falsify-congelaram-em-2026-09-08-e-106-rotulos-caem-no-peso-pessimista-porque-a-calibracao-exige-um-passo-manual-que-o-ci-nunca-faz.md
