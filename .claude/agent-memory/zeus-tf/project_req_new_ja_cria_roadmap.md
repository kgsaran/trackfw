---
name: req-new-ja-cria-roadmap
description: trackfw req new já cria a roadmap vinculada; rodar roadmap new com o mesmo título depois a sobrescreve sem vínculo e só o push pega
metadata:
  type: project
---

`trackfw req new "<título>"` já cria a roadmap em `backlog/` **vinculada** à REQ. Rodar em seguida
`trackfw roadmap new "<mesmo título>"` (como o protocolo do CLAUDE.md ainda manda) **sobrescreve** essa roadmap
em silêncio, com `req: ""` e `REQ: ` vazios. O `validate` só avisa; o hard gate do `trackfw push` barra
("roadmap in wip but has no linked REQ") — dois commits depois. Medido em 2026-10-08 (REQ-2026-10-08).

**Why:** o defeito é do produto (mesmo mecanismo da REQ-2026-09-09, "REQ nasce órfã") e o protocolo escrito
ainda induz a ele.

**How to apply:** depois do `req new`, NÃO rode `roadmap new`; confira `req:` no frontmatter da roadmap antes do
primeiro commit. Se o KG quiser, vira ML na REQ-2026-09-09. Ver [[push-e-pr-via-ship]].
