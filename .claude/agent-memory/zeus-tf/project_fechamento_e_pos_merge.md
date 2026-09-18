---
name: fechamento-e-pos-merge
description: O roadmap fica em wip/ até o merge — trackfw push tem hard gate exigindo REQ + roadmap em wip/; mover para done antes do PR se auto-bloqueia
metadata:
  type: project
---

**O fechamento de governança (`roadmap move done` + REQ `status: Done`) é pós-merge, não pré-PR.**

**Why:** `trackfw push --help` declara, na própria ajuda, *"Validates governance — REQ + roadmap in
wip/ must exist for feat/fix/refactor branches (hard gate: not affected by lenient mode or per-rule
severity)"*. Mover o roadmap para `done/` esvazia `wip/` e o push seguinte é recusado — o fechamento
antecipado bloqueia a própria entrega. Medido em 2026-09-18 no PR #393.

**How to apply:** na ordem de encerramento de uma REQ —
1. commitar + `trackfw push` + `gh pr create` **com o roadmap ainda em `wip/`**;
2. só depois do merge: `trackfw roadmap move <nome> done`, REQ para `status: Done`, **e atualizar o
   ponteiro `wip/` → `done/`** na linha `Roadmap:` da seção `## Linked Roadmap` da REQ — senão
   `ref_targets_exist` passa a violar.

**Armadilha correlata, mesma sessão:** o marcador que o validador enxerga é o de **início de linha**
(`Roadmap: \`caminho\``, dentro de `## Linked Roadmap`). O bloco de cabeçalho em negrito
(`**Roadmap:** ...`) **não conta** — `req_roadmap_sync` avisa "has no linked Roadmap (marker must
start the line)". O `req new` não gera a seção `## Linked Roadmap`; gera só `## Linked ADR`. Conferir
`trackfw validate 2>&1 | grep <nome-da-REQ>` depois de criar a REQ.

Ver [[o-instrumento-mente]] e [[palavra-chave-de-fechamento-e-em-ingles]].
