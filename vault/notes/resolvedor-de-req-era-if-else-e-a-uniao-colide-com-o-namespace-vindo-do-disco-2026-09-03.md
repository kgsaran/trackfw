---
title: O resolvedor de REQ das regras era if/else (4 dos 6 layouts vácuos, não 1), e transformá-lo em união colide com agents:∪disco — dedup é obrigatório
tags: [validator, by_agent, req_dir, roadmap_namespacing, uniao, dedup, traceid, paridade]
date: 2026-09-03
related: [[uniao-disco-agents-mascara-gate-por-presenca-2026-08-29]], [[validate-parity-gate-vacuo-e-go-sem-helper-unico-2026-08-01]], [[req-move-statedir-hardcoded-roadmapdir-2026-08-04]]
---

## 1. A leitura de backlog subestimou o defeito, e a ADR só o corrigiu pela metade

A ADR-2026-09-03 corrigiu a estimativa "é fiação, não lógica nova" para "falta **um** caso em
`listREQFiles`". Medido antes de qualquer edição, com a fixture do relato (REQ com `adr:`/`roadmap:`
apontando para alvos inexistentes) e contando violações de `validate` nos 3 CLIs:

```
layout                          go  node  py
flat      req_dir/*.md           2    2    2
flat      req_dir/<estado>/      0    0    0
by_agent  req_dir/*.md           0    0    0
by_agent  req_dir/<estado>/      0    0    0
by_agent  req_dir/<agente>/      0    0    0   <- canônico
by_agent  req_dir/<agente>/<estado>/  2  2  2
```

**Quatro dos seis layouts eram vácuos.** A razão: `listREQFiles` (generators) era união de 3 casos,
mas **não é a função que as regras usam**. As regras usavam `resolveREQFiles`/`resolveReqFiles`/
`resolve_req_files`, que era um `if/else` — `by_agent` ⇒ *só* `<agente>/<estado>/`; senão *só* flat.
Auditar a função de nome parecido dá o veredito errado; conte pelos **call sites das regras**.

Inventário real do que precisava convergir: **9 implementações de leitura** (3 runtimes × validator +
generators/req + generators/roadmap-ou-context) e **3 pontos de escrita**.

## 2. `traceid` não passava pelo resolvedor — recebia um DIRETÓRIO

`validateTraceId` (Go) e `check_traceid` (Python) montavam a própria árvore a partir de `req_dir`.
Corrigir só o resolvedor **não** as alcançaria: continuariam vácuas em `by_agent` canônico, e a AC
"zero regras enxergando zero REQs" reprovaria por um motivo fora do resolvedor. O Node, por acaso,
usava `walkMd` recursivo e já era um superconjunto — três runtimes, três semânticas.

**Regra de bolso:** ao consertar um resolvedor, procure quem recebe **diretório** em vez da lista
resolvida. Esses são os que sobrevivem ao conserto.

## 3. O achado não previsto: a união COLIDE consigo mesma

`resolveAgentNamespaces` devolve `agents:` **∪ disco** (REQ-2026-08-29 — ver
[[uniao-disco-agents-mascara-gate-por-presenca-2026-08-29]]), filtrando só `node_modules`. Logo, um
`req_dir/backlog/` real **é reportado como agente**, e o caso novo `req_dir/<agente>/*.md` emite
exatamente os mesmos paths do caso `req_dir/<estado>/*.md`.

Antes da união não havia interseção entre os casos, então ninguém deduplicava. Ao acrescentar o
canônico, toda REQ em layout por-estado passaria a ser contada **duas vezes** — e cada violação sairia
**em duplicata**, o que também estragaria a AC numérica ("2 violações" viraria 4).

Dedup é por **caminho normalizado** (`filepath.Clean` / `path.normalize` / `os.path.normpath`).
🔴 **Não** deduplicar filtrando nomes de estado da lista de agentes: um agente legitimamente chamado
`done` existe e desapareceria — além de reabrir o resolvedor de namespace do PR #218, que está
correto.

Corolário: a lista resolvida precisa ser **ordenada** no ponto único. A varredura é agent-major e
`fs.readdirSync` não garante ordem — sem `sort`, os 3 runtimes divergem por filesystem.

## 4. Como medir "a regra não fica mais vácua" sem se enganar

Contar violações não basta: numa fixture com roadmaps, `ref_targets_exist` e `traceid` **disparam pelo
lado do roadmap** mesmo com zero REQs lidas. Ao sabotar de volta o caso canônico, a fixture de 6
regras mostrou `vistas=2 (ref_targets_exist, traceid) | zero=4`. Se a métrica fosse "a regra
apareceu na saída", duas regras dariam falso verde.

A métrica que discrimina é **por artefato**: as violações citam o **arquivo de REQ**. Uma asserção
honesta é "cada regra produz ao menos uma violação cujo `file` é uma REQ".

## 5. Teste que testava uma CÓPIA

`npm/tests/context_req_by_agent.test.js` reimplementava `collectEntries`/`collectReqs` do
`context.js` dentro do próprio arquivo de teste. Ele passava verde **independentemente** do que a
produção fizesse — inclusive com o `context` sem enxergar o layout canônico. Mesmo padrão de vacuidade
descrito em [[validate-parity-gate-vacuo-e-go-sem-helper-unico-2026-08-01]]. Fechado chamando a função
de produção; se outro teste "espelha a lógica" em comentário, desconfie: espelho não é chamada.

## 5-bis. Métrica: "a regra disparou" mente; "a violação nomeia uma REQ" não

A forma fraca (`a regra apareceu na saída`) dá **6/6 verde mesmo na árvore sabotada** em Go, porque
`ref_targets_exist` e `traceid` disparam pelo lado do **roadmap**. A forma que discrimina exige que a
violação nomeie um basename `REQ-*.md` — no campo `file` **ou** na `message`, porque em Go
`blocked_by_draft_adr` sai com `file: ""` e nomeia a REQ só no texto. Com a métrica forte:
árvore boa 6/6 nos 3 CLIs e nos 6 layouts; árvore sabotada 1/6 (Go, Python) e 2/6 (Node).

## 6. Resíduos conhecidos (não corrigidos aqui, de propósito)

- **`traceid` do CLI Node continua fora do ponto único**: `npm/src/validator/traceid.js` indexa REQ
  com varredura **recursiva** (`walkMd`) — superconjunto dos 4 layouts, nunca vácuo, mas é a segunda
  noção de layout dentro daquele runtime. Prova empírica da divergência: sabotando o caso canônico
  nos 3 resolvedores, Go e Python caem para **5** regras sem violação nomeando REQ, e o Node cai para
  **4** — a diferença é exatamente esse resíduo. Convergir **estreita** o Node ao contrato e obriga os
  testes de traceid a declarar `roadmap_namespacing`.
- `trackfw sync` **hardcoda `docs/req`** nos 3 CLIs (`npm/src/commands/sync.js:237`,
  `pypi/trackfw/commands/sync.py:197`, `internal/commands/sync.go`) — ignora `req_dir` do
  `trackfw.yaml` inteiramente. Defeito próprio, fora do escopo desta REQ.
- `req move` continua movendo REQ para `req_dir/<agente>/<estado>/`, o que contraria o invariante D1
  (REQ não tem dimensão de estado). A leitura tolera; a escrita do `move` não foi tocada porque o
  escopo negativo da ADR proíbe introduzir/alterar partição por estado nesta REQ.
- Go e Python divergem do Node na semântica de `traceid_state_mismatch` para REQ (Go compara pasta;
  Python compara `status:` do frontmatter). Divergência pré-existente, não tocada.
