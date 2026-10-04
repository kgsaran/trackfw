---
title: reqFilenameDateRe não casa roadmaps — D5 precisa de roadmapCreationDate separada
date: 2026-10-04
tags: [validator, cutoff, d5, wave0, deadlock]
---

# reqFilenameDateRe não casa roadmaps — D5 precisa de `roadmapCreationDate` separada

## O que foi encontrado

`reqFilenameDateRe = regexp.MustCompile('^REQ-(\d{4}-\d{2}-\d{2})')` usa âncora `^` e
prefixo `REQ-`. Não casa:
- `ROADMAP-2026-09-09-slug.md` (prefixo errado)
- `v2.0-gaps-implementacao-2026-06-13.md` (data no sufixo)

## Impacto para a D5

33 roadmaps em `done/` com data no sufixo sem `date:` no frontmatter (todos datados
2026-06-11 a 2026-09-09 < corte 2026-09-18). Se reativados em `wip/` pela Regra Dura:
- `roadmapCreationDate` ilegível → fail-closed → tratados como pós-corte
- `roadmap_wave0_required` dispara → Wave 0 exigida → deadlock recriado para esses roadmaps

## Mitigação obrigatória para ML-1B

Criar `roadmapCreationDate` separada de `reqCreationDate`, com regex que cubra:
1. `date:` frontmatter (primária)
2. `ROADMAP-YYYY-MM-DD` prefixo
3. `*-YYYY-MM-DD.md` sufixo (para os 33 roadmaps antigos)

Alternativa simples: `regexp.MustCompile('(\d{4}-\d{2}-\d{2})')` (primeira data no basename).
Declare o regex escolhido no docblock com o número de roadmaps cobertos por cada ramo.

## Referência

REQ-2026-10-04, ML-0A (`docs/seguranca/2026-10-04-wave0-criterio-caducado-e-cortes.md`, seção T7/R6).
