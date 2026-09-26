---
name: fixture-de-req-precisa-alvo-em-abandoned
description: Alvo de vínculo Roadmap em fixture do check-gates-falsify vai em docs/roadmaps/abandoned/ — backlog/ colide com o `find docs/roadmaps/backlog` dos Cenários 24/25/26, wip/ dispara 3 regras e done/ dispara req_roadmap_lifecycle
metadata:
  type: feedback
---

Quando uma fixture de REQ do `check-gates-falsify.sh` precisar de um alvo `Roadmap:` **real**, crie-o
em `docs/roadmaps/abandoned/`.

**Why:** medido em 8 células (REQ Open × REQ Done × 4 estados) em 2026-09-26. `wip/` dispara
`wip_wave0` + `wip_has_req` + `wip_acceptance`; `done/` dispara `req_roadmap_lifecycle` para REQ Open;
`backlog/` é limpo **mas** os scripts de ciclo dos Cenários 24/25/26 fazem
`basename "$(find docs/roadmaps/backlog -name "*.md")"` — um segundo arquivo ali devolve duas linhas e
o `roadmap move` recebe nome corrompido. `abandoned/` é limpo nos dois status de REQ e não é destino de
comando nenhum do harness.

**How to apply:** use `ensure_roadmap_link_target` (o helper já existe, com o porquê escrito). O
frontmatter do alvo tem de declarar `status: abandoned` (a pasta é a fonte de verdade do estado). E o
alvo tem de **existir no disco**: `ref_targets_exist` está em `lenientCarveoutRules`, logo é violação
mesmo em `lenient` — apontar para um `.md` inexistente troca um defeito por outro.
Relacionado: [[migrar-leitor-de-regra-esvazia-falsify]].
