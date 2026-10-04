---
status: Done
date: 2026-10-04
author: "zeus-tf"
adr: "docs/adr/ADR-2026-10-04-criterio-de-aceite-caducado-e-marcado-como-tal-com-justificativa-e-regras-novas-sobre-artefato-antigo-tem-corte-por-data-declarado.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md"
---

# REQ: nao ha como fechar REQ com criterio de aceite permanentemente inverificavel sem afirmar algo falso, e a exigencia de Wave 0 vale para roadmap anterior a ADR que a criou

> Date: 2026-10-04 | Status: Done
| GitHub Issue: #514

## Motivation

Critério que ficou permanentemente inverificável (o artefato foi removido, ou o número literal caducou)
não tem forma honesta de fechar. Medido com o binário da `main`:
- o `barrier` bloqueia ML `✅` que tem `- [ ]` aberto;
- o `move … done` bloqueia ML `❌`.

O caminho que passa é marcar `[x]` falso. Além disso, a exigência de `## Wave 0` vale para roadmap
anterior à ADR que a criou: 4 roadmaps daqui não chegam a `done/`. Decisões no ADR ligado: nenhum estado
novo, uma linha `Caducou:` com justificativa, e cortes por data declarados.

## Acceptance Criteria

- [x] **AC1** — 🔴 **Wave 0:** threat model, com parecer em `docs/seguranca/`. Deve cobrir:
  - a enumeração de **todo** parser de caixa de critério (`internal/`, `scripts/`, testdata e o board do
    `serve`), com o que cada um faz hoje com `- [ ]`;
  - `Caducou:` vazio, em comentário, dentro de cerca de código, ou colado num item que não é critério;
  - `Caducou:` usado para esconder critério que dava para verificar;
  - a data retroativa para escapar da D4 e da D5;
  - a interação com o gate de placeholder de scaffold e com o falsify do gate do `done`.
      ✅ Evidência: `docs/seguranca/2026-10-04-wave0-criterio-caducado-e-cortes.md`: 7 parsers enumerados; T3, T7 e T8 incorporados ao ADR como D6
- [x] **AC2** — D3: o `barrier` não conta como "unmet" um critério aberto com `Caducou:` justificado; ele
  aparece como "lapsed" no texto e no JSON. Critério aberto sem justificativa continua bloqueando. Fixture
  de ponta a ponta com o binário.
      ✅ Evidência: testes `TestBarrierLapsed_*` e `TestAcceptance*Lapsed*`; fixture de ponta a ponta: ML com `[x]` + `[ ]`/`Caducou:` mostra "~ 1 lapsed" e a justificativa (`lapsed_details` no JSON)
- [x] **AC3** — D2: `Caducou:` sem texto, fora da linha de continuação do item, ou dentro de cerca de
  código **não** conta como caducado.
      ✅ Evidência: testes de não-contar (vazio, sem indentação, depois de linha em branco, em cerca, em HTML); revisão Wave 2: 17 vetores sem bypass
- [x] **AC4** — D4: a regra `req_done_open_criteria` (warning) dispara para REQ `Done` com `- [ ]` sem
  `Caducou:` e data ≥ 2026-10-04. REQ anterior → isenta, numa linha agregada com a contagem. Medido neste
  repositório: 0 avisos individuais, e uma linha agregada com as isentas (126, medido pelo validator).
      ✅ Evidência: `validate` neste repositório: linha agregada com 126 de 211 REQs Done isentas e 0 cobradas; `TestReqDoneOpenCriteria_*`, com mordida (S1/S2)
- [x] **AC5** — D5: roadmap com data < 2026-09-18 sem `## Wave 0` vai para `done/` com uma linha de
  isenção. Com data ≥ 2026-09-18, continua bloqueado. O mesmo corte vale no `roadmap_wave0_required`.
  Medido: os 4 roadmaps daqui sem Wave 0 fora de `done/` passam no gate em dry-run, ou num teste com
  cópia.
      ✅ Evidência: `TestMoveDoneWave0Cutoff_*` (move real em TempDir) e `TestRoadmapWave0Cutoff_*`; os 4 roadmaps sem Wave 0 testados numa cópia passam a ter a linha "Wave 0 not required"
- [x] **AC5b** — D6/T3: caixa com caractere fora de ` `/`x`/`X` conta como pendente, e o `barrier` nomeia
  a linha. D6/T8: um ML só com critérios caducados (0 atendidos) fica `blocked`. D6/T7: a data do
  roadmap vem de `date:` ou da primeira `AAAA-MM-DD` do nome, incluindo o formato com data no sufixo.
      ✅ Evidência: `[~]`/`[-]` → unmet com "unrecognized checkbox"; ML só de caducados → "all acceptance criteria lapsed"; `RoadmapCreationDate` lê a data do sufixo
- [x] **AC6** — `docs/cli-parity.md` documenta a forma `Caducou:`, a regra nova e os dois cortes, com
  `trackfw-contract`.
      ✅ Evidência: `docs/cli-parity.md`: três seções novas com `trackfw-contract`, incluindo a mudança de semântica do `unmet` no `show --json`
- [x] **AC7** — Os templates de roadmap e REQ gerados não mudam de forma. A documentação do que fazer com
  critério que caducou fica no `cli-parity.md` e nas instruções de governança geradas, se elas tratarem
  de fechamento.
      ✅ Evidência: os templates gerados não mudaram (`git diff origin/main...HEAD --stat` do PR #519 sem arquivo de template)
- [x] **AC8** — Cada teste novo declara o que afirma e reprova sem a correção.
      ✅ Evidência: uma frase por teste nos relatórios; mordida provada por sabotagem de produção (S1–S6 no ML-1C, S1 no ML-1D)
- [x] **AC9** — `make quality` EXIT=0 com a máquina ociosa; CI verde, inclusive `windows-full-suites`.
      ✅ Evidência: `make quality` EXIT=0 (2933 linhas, falsify 347 OK / 0 FAIL); CI do PR #519: 20/20

## Negative scope

- **Estado novo de roadmap.**
- **Editar as REQs `Done` existentes** com caixa aberta.
- **Token novo de caixa** (`[~]` e afins).
- **Verificar se a justificativa é verdadeira.**
- **Endurecer `req_done_open_criteria` para erro.**

## Linked ADR
ADR: docs/adr/ADR-2026-10-04-criterio-de-aceite-caducado-e-marcado-como-tal-com-justificativa-e-regras-novas-sobre-artefato-antigo-tem-corte-por-data-declarado.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/done/ROADMAP-2026-10-04-nao-ha-como-fechar-req-com-criterio-de-aceite-permanentemente-inverificavel-sem-afirmar-algo-falso-e-a-exigencia-de-wave-0-vale-para-roadmap-anterior-a-adr-que-a-criou.md
