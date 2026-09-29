# Wave 2 — Auditoria Independente: `traceid_orphan_req` reprova estado correto

> Data: 2026-09-29 | Autor: hades-tf | Branch: fix/orphan-req-reprova-estado-correto

**Método:** reimplementação da expectativa a partir da ADR e da REQ vigente, medição do binário compilado como caixa-preta em fixtures no scratchpad. Nenhuma leitura de diff ou de testes dos executores anteriores.

Binário: `trackfw 9.1.0`, compilado em `2026-09-29` em `/Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw`.
`git diff trackfw.yaml`: vazio.

Fixtures em scratchpad:
- `ml2a-fixture/` — layout plano, verificações iniciais
- `ml2a-fix/` — pós-cutoff date, A1/A2 deconfounded, D4 ghost + lifecycle
- `ml2a-d1bis/` — discriminador D1-bis
- `ml2a-byagent/` — layout by_agent, C2 e contra-braço

---

## Medição 1 — C1: REQ `Open` com `req_id`, sem roadmap

**ADR/REQ esperam:** C1 não dispara `traceid_orphan_req` após a correção (D2: só `Done` dispara).

**Fixture:** `REQ-C1-open-no-roadmap.md` — `status: Open`, `req_id: "REQ-C1"`, sem `roadmap:`. Layout plano.

**Saída literal:**
```
(nenhuma linha traceid_orphan_req para REQ-C1)
req_has_roadmap: não dispara (Open não é Done)
```

**Veredito:** C1 FECHADA. `traceid_orphan_req` e `req_has_roadmap` concordam: REQ `Open` sem roadmap não é violação.

---

## Medição 2 — C2: par válido (REQ Open, roadmap com `req:` mas sem `req_id`)

**ADR/REQ esperam:** C2 não dispara `traceid_orphan_req` após D3 (casamento via campo `req:` do roadmap).

**Fixture:**
- `REQ-C2-open-with-paired-roadmap.md`: `status: Open`, `req_id: "REQ-C2"`, `roadmap: docs/roadmaps/wip/ROADMAP-C2-paired.md`
- `ROADMAP-C2-paired.md`: `req: "docs/req/REQ-C2-open-with-paired-roadmap.md"`, SEM campo `req_id`

**Saída literal:**
```
(nenhuma linha traceid_orphan_req para REQ-C2)
req_has_roadmap: não dispara (roadmap: preenchido com caminho real)
```

**Veredito:** C2 FECHADA. D3 funciona: casamento via `req:` do roadmap satisfaz `traceid_orphan_req` sem exigir `req_id` no frontmatter do roadmap.

---

## Medição 3 — Contra-braço: REQ `Done` sem roadmap (caminho enforced)

**ADR/REQ esperam:** AINDA dispara nas duas regras. Se nada disparar, a correção virou remoção. Medição anterior usou `date: 2026-01-01` (pré-cutoff) — apenas grandfathering, não o caminho enforced. Esta medição usa `date: 2026-09-29` (pós-cutoff 2026-09-03).

**Fixture:** `REQ-2026-09-29-done-no-roadmap.md` — `status: Done`, `date: 2026-09-29`, `req_id: "REQ-POST"`, sem `roadmap:`.

**Saída literal:**
```
req_has_roadmap grandfathering: 0 REQ(s) exempt, 4 enforced, 7 REQ(s) scanned
✗ req "REQ-2026-09-29-done-no-roadmap.md" has no linked Roadmap
✗ traceid_orphan_req: req "REQ-2026-09-29-done-no-roadmap.md" has req_id="REQ-POST" but no Roadmap with same id
```

**Veredito:** Contra-braço PRESERVADO no caminho enforced. Ambas as regras disparam como `✗` (error, não grandfathered warning). A correção não desligou a regra.

---

## Medição 4 — `Superseded` e `Closed` (ADR D2-bis)

**ADR/REQ esperam:** não disparam.

**Fixtures:**
- `REQ-SUPERSEDED.md` — `status: Superseded`, `req_id: "REQ-SUP"`, sem roadmap
- `REQ-CLOSED.md` — `status: Closed`, `req_id: "REQ-CLO"`, sem roadmap

**Saída literal:**
```
(nenhuma linha traceid_orphan_req ou req_has_roadmap para esses dois)
```

**Veredito:** D2-bis SATISFEITA. `Superseded` e `Closed` não disparam em nenhuma das duas regras.

---

## Medição 5 — Grafias vizinhas do status `Done`

**ADR/REQ esperam:** `EqualFold` → `done`, `Done`, `DONE` disparam. `"Done"` (quoted) e `"Done "` (trailing space) não declarados. Fixtures com `date: 2026-09-29` (pós-cutoff) para caminho enforced. Controle positivo: `ROADMAP-has-req-id.md` com `req_id: "REQ-DUNQ"` confirma que o índice funciona (`trace_id_field is set but Roadmaps (0) were indexed` desaparece do output).

| fixture | status exato | traceid_orphan_req | req_has_roadmap (enforced) |
|---|---|---|---|
| `REQ-done-unquoted.md` | `Done` (sem aspas) | não dispara¹ | DISPARA ✗ |
| `REQ-done-quoted.md` | `"Done"` (aspas, sem espaço) | DISPARA ✗ | DISPARA ✗ |
| `REQ-done-quoted-trailing-space.md` | `"Done "` (aspas + espaço) | não dispara | não dispara |

¹ `traceid_orphan_req` não dispara para `REQ-done-unquoted.md` porque `ROADMAP-has-req-id.md` tem `req_id: "REQ-DUNQ"` — controle positivo do índice. Ver Achado D4.

**Saída literal (relevante):**
```
✗ traceid_orphan_req: req "REQ-done-quoted.md" has req_id="REQ-DQUO" but no Roadmap with same id
✗ req "REQ-done-unquoted.md" has no linked Roadmap (marker...) [req_has_roadmap - enforced]
✗ req "REQ-done-quoted.md" has no linked Roadmap (marker...) [req_has_roadmap - enforced]
(nada para REQ-done-quoted-trailing-space.md em traceid_orphan_req ou req_has_roadmap)
```

**Veredito:**
- `"Done"` (quoted, sem espaço): `strings.EqualFold("Done", "done")` = true → dispara em ambas as regras. Comportamento correto.
- `"Done "` (trailing space): `strings.EqualFold("Done ", "done")` = false → NÃO dispara em nenhuma das regras. Bypass silencioso documentado; existia antes da correção; não é regressão.

Nota: `Done ` (espaço sem aspas) não foi testado separadamente — YAML costuma strip trailing whitespace de strings não-quotadas, tornando esse caso equivalente a `Done`. O caso relevante é o quotado, que foi medido.

---

## Medição 6 — D1-bis: flat layout vs. subdir layout

**ADR/REQ esperam:** a regra usa `status:` do frontmatter (sempre disponível), não `e.state` (vazio em layout plano; nome do diretório em subpastas). Discriminador: REQ com `status: Done` fisicamente em `docs/req/backlog/` — se a regra usa `e.state` (= "backlog"), não dispara; se usa `status:`, dispara.

**Fixture (`ml2a-d1bis/`):**
- `docs/req/backlog/REQ-2026-09-29-done-status-in-backlog-dir.md` — `status: Done`, `date: 2026-09-29`, sem roadmap
- Controle: `docs/req/backlog/REQ-2026-09-29-open-in-backlog.md` — `status: Open`, mesmo dir

**Saída literal:**
```
req_has_roadmap grandfathering: 0 exempt, 1 enforced, 2 REQ(s) scanned
✗ traceid_orphan_req: req "REQ-2026-09-29-done-status-in-backlog-dir.md" has req_id="REQ-D1BIS" but no Roadmap with same id
(nada para REQ-2026-09-29-open-in-backlog.md)
```

**Veredito:** D1-bis CONFIRMADA. A regra USA `status:` do frontmatter. REQ com `status: Done` em `backlog/` dispara — se usasse `e.state` (= "backlog"), não dispararia. O subdir físico da REQ é irrelevante para a decisão. Comportamento idêntico em layout plano e em subpastas.

---

## Medição 7 — Limite de D3: casos fora do alcance

**ADR declara** 58 roadmaps fora do alcance de D3: 32 `req:` stale · 8 vazio/null · 18 sem campo.

### A1 — Stale path (diretório migrado) — deconfounded

**Fixture original (confundida):** `REQ-D3-stale.md` tinha `roadmap:` campo apontando para o roadmap, o que satisfazia `req_has_roadmap` independentemente da stale path.

**Fixture deconfounded:** `REQ-2026-09-29-stale-test.md` — `status: Done`, `date: 2026-09-29`, sem campo `roadmap:`. Roadmap tem `req: "docs/requisições/REQ-2026-09-29-stale-test.md"` (diretório antigo).

**Saída literal:**
```
✗ roadmap "ROADMAP-stale-test.md" links to REQ "docs/requisições/REQ-2026-09-29-stale-test.md"
    but the file was found at docs/req/REQ-2026-09-29-stale-test.md (stale path)  [ref_targets_exist]
✗ req "REQ-2026-09-29-stale-test.md" has no linked Roadmap [req_has_roadmap - enforced]
(nenhuma linha traceid_orphan_req para REQ-2026-09-29-stale-test.md)
```

**Achado — A1 confirmado, NÃO é confusão de fixture.** `traceid_orphan_req` NÃO dispara mesmo sem `roadmap:` na REQ. D3 herda o stale-path fallback do validador: quando `req:` do roadmap aponta para `docs/requisições/X.md` e o arquivo existe em `docs/req/X.md`, o validador encontra o arquivo e D3 o aceita como match.

Consequência para o limite declarado na ADR: para a subcategoria de renomeação de diretório (`docs/requisições/` → `docs/req/`), quando o arquivo existe no novo path, D3 + stale fallback pode resolver o caso. O limite de "32 stale" na ADR pode estar overcounted para essa subcategoria.

Limite desta inferência: a inferência vem de uma fixture sintética, não do corpus real. No repositório consumidor, `docs/requisições/` é o `req_dir` atual — não um caminho stale — então as 17 ocorrências são de outro tipo de staleness. O discriminador correto é rodar `./bin/trackfw validate | grep traceid_orphan_req` em uma cópia do corpus do consumidor com `trace_id_field: req_id` ativado — o que não pode ser feito a partir deste repositório. A inferência de "17 podem estar resolvidos" está fora do alcance confirmável aqui.

Nota colateral: `req_has_roadmap` ainda dispara (REQ sem `roadmap:` campo). Isso revela uma assimetria de D3: a correção foi aplicada a `traceid_orphan_req` (via stale fallback), mas `req_has_roadmap` não tem o mesmo mecanismo de match reverso — ver Achado D4 abaixo.

### B — `req:` vazio/null

**Saída literal:**
```
✗ traceid_orphan_req: req "REQ-D3-empty-req.md" has req_id="REQ-D3E" but no Roadmap with same id
```

**Veredito:** DISPARA como esperado. D3 não pode casar com `req:` vazio.

### C — Sem campo `req:`

**Saída literal:**
```
✗ traceid_orphan_req: req "REQ-D3-no-req-field.md" has req_id="REQ-D3N" but no Roadmap with same id
```

**Veredito:** DISPARA como esperado. D3 não pode casar com roadmap sem `req:`.

---

## Medição 2b — C2 em layout `by_agent` (consumidor real)

**ADR/REQ esperam:** a correção que funciona em layout plano deve funcionar também em `by_agent` com roadmaps em `<roadmap_dir>/<agent>/wip/`. Sem esta medição o gate é vacuoso para o consumidor, que usa `roadmap_namespacing: by_agent` (citado na própria REQ como lição do issue #396).

**Fixture (`ml2a-byagent/`):** `roadmap_namespacing: by_agent`, `agents: [apolo]`, `trace_id_field: req_id`.
- C2 arm: `REQ-2026-09-29-c2-by-agent.md` — `status: Open`, `req_id: "REQ-BA-C2"`, roadmap em `docs/roadmaps/apolo/wip/ROADMAP-by-agent-c2.md` com `req:` back-link e sem `req_id`
- Controle: `REQ-2026-09-29-done-no-roadmap-ba.md` — `status: Done`, `req_id: "REQ-BA-DONE"`, sem roadmap — DEVE disparar

**Saída literal:**
```
req_has_roadmap grandfathering: 0 exempt, 1 enforced, 2 REQ(s) scanned
trace_id_field is set but Roadmaps (0) were indexed while REQs (2) were
✗ traceid_orphan_req: req "REQ-2026-09-29-done-no-roadmap-ba.md" has req_id="REQ-BA-DONE" but no Roadmap with same id
(nenhuma linha traceid_orphan_req para REQ-2026-09-29-c2-by-agent.md)
```

O `Roadmaps (0) were indexed` é esperado: nenhum roadmap tem `req_id` (trace_id_field) no frontmatter — D3 opera via `req:`, não via índice. O controle dispara → o rule chain está ativo; o silêncio para C2 é filtro de status, não ausência de execução.

**Veredito:** C2 FECHADA em `by_agent`. O indexer percorre `apolo/wip/` e D3's `req:` resolution funciona no nível adicional de subpasta.

---

## Medição 8 — D4: as 4 regras aplicam critério consistente

**ADR/REQ esperam:** `req_has_roadmap`, `traceid_orphan_req`, `ref_targets_exist`, `req_roadmap_lifecycle` concordam sobre o mesmo par.

### 8a — Core cases (C1, C2, Done-no-roadmap)

| par | traceid_orphan_req | req_has_roadmap | concordam? |
|---|---|---|---|
| Open sem roadmap (C1) | não dispara | não dispara | SIM ✓ |
| Open com roadmap válido via req: (C2) | não dispara | não dispara | SIM ✓ |
| Done sem roadmap (pós-cutoff) | DISPARA ✗ | DISPARA ✗ (enforced) | SIM ✓ |
| Superseded sem roadmap | não dispara | não dispara | SIM ✓ |

**Veredito:** Para as causas que a REQ trata, D4 está satisfeita.

### 8b — `ref_targets_exist` não tem filtro de status

**Fixture:** `REQ-2026-09-29-open-ghost-roadmap.md` — `status: Open`, `roadmap: "docs/roadmaps/wip/ROADMAP-NONEXISTENT-GHOST-FILE.md"` (arquivo não existe).

**Saída literal:**
```
✗ req "REQ-2026-09-29-open-ghost-roadmap.md" links to Roadmap
    "docs/roadmaps/wip/ROADMAP-NONEXISTENT-GHOST-FILE.md" which does not exist
(nenhuma linha traceid_orphan_req — Open, passado por D2)
(nenhuma linha req_has_roadmap — Open, passado por D2)
```

`ref_targets_exist` dispara para REQ Open com ghost roadmap. `req_has_roadmap` e `traceid_orphan_req` não disparam. A Wave 0 já documentou essa divergência. Não é regressão introduzida pela correção — `ref_targets_exist` sempre foi status-agnostic e está perguntando outra coisa ("o caminho declarado existe?", não "deveria ter roadmap?"). O ML-1A é responsável por endereçar ou declarar explicitamente este comportamento.

### 8c — `req_roadmap_lifecycle` já filtra por status Open

**Fixture:** `REQ-2026-09-29-open-roadmap-in-done.md` — `status: Open`, roadmap em `done/`.

**Saída literal:**
```
✗ req "REQ-2026-09-29-open-roadmap-in-done.md" is Open but linked Roadmap
    "docs/roadmaps/done/ROADMAP-lifecycle-done.md" is in done/
```

**Veredito:** `req_roadmap_lifecycle` dispara corretamente. Já tinha filtro de status — continua funcionando.

### 8d — Achado D4: assimetria de D3 entre `traceid_orphan_req` e `req_has_roadmap`

**Fixture:** `REQ-done-unquoted.md` — `status: Done`, sem `roadmap:` campo. Roadmap separado `ROADMAP-has-req-id.md` com `req_id: "REQ-DUNQ"` E `req: "docs/req/REQ-done-unquoted.md"`.

**Saída literal:**
```
✗ req "REQ-done-unquoted.md" has no linked Roadmap [req_has_roadmap - enforced]
(nenhuma linha traceid_orphan_req para REQ-done-unquoted.md)
```

Controle positivo confirmado: o roadmap foi indexado — a mensagem `trace_id_field is set but Roadmaps (0) were indexed` desapareceu do output desta fixture.

**Achado:** `traceid_orphan_req` PASSA (roadmap tem `req_id: "REQ-DUNQ"` → match via índice). `req_has_roadmap` DISPARA (REQ não tem campo `roadmap:`). As duas regras divergem para o caso em que a ligação existe de roadmap → REQ mas não de REQ → roadmap.

Este é um estado possível no produto: o usuário cria um roadmap com `req_id:` correto mas não atualiza o `roadmap:` da REQ. D3 corrigiu `traceid_orphan_req` para aceitar o link reverso (`req:` do roadmap), mas `req_has_roadmap` ainda exige o link direto (`roadmap:` da REQ). A assimetria é nova no sentido em que D3 aumentou a capacidade de `traceid_orphan_req` sem atualizar `req_has_roadmap` para o mesmo nível.

Severidade: **gap de D4**, não regressão. No fluxo normal do produto (`req new` → `roadmap new`), o campo `roadmap:` da REQ é populado quando o roadmap é criado, então o caso de divergência é incomum. Mas a ADR D4 afirma que as regras devem concordar — e aqui não concordam para um estado legítimo do sistema.

---

## Veredito final

### O que a correção fechou e preservou

| critério | ADR/REQ esperam | medido | status |
|---|---|---|---|
| C1: REQ Open sem roadmap | não dispara | não dispara | FECHADA ✓ |
| C2: par via req: (sem req_id no roadmap) — flat | não dispara | não dispara | FECHADA ✓ |
| C2: par via req: (sem req_id no roadmap) — by_agent | não dispara | não dispara | FECHADA ✓ |
| Contra-braço Done sem roadmap (enforced, pós-cutoff) | dispara como ✗ | dispara como ✗ em ambas as regras | PRESERVADO ✓ |
| Superseded/Closed | não disparam | não disparam | CORRETO ✓ |
| `"Done"` quoted (sem espaço) | dispara | dispara | CORRETO ✓ |
| D1-bis: flat = subdir (usa status: não e.state) | comportamento idêntico | idêntico | CONFIRMADO ✓ |
| D3 empty req: / sem campo req: | ainda disparam | ainda disparam | CORRETO ✓ |
| req_roadmap_lifecycle | filtra Open corretamente | filtra | CORRETO ✓ |
| **ADR D4: as 4 regras aplicam critério consistente** | concordam | **NÃO** — 2 gaps medidos | **NÃO SATISFEITO** |

### Achados

**A1 — D3 resolve stale path de renomeação de diretório (ADR overcounts limit)**
Deconfounded: mesmo sem `roadmap:` na REQ, `traceid_orphan_req` não dispara quando o roadmap tem `req:` com stale path e o arquivo existe no novo path. O fallback stale do validador beneficia D3. O limite de "32 stale" na ADR pode incluir casos que D3 já resolve para a subcategoria de renomeação de diretório. Discriminador a rodar no consumidor: `./bin/trackfw validate | grep traceid_orphan_req`.
Severidade: **informativa** — o comportamento é melhor do que o declarado.

**A2 — `"Done "` (aspas + espaço final) bypassa ambas as regras**
`strings.EqualFold("Done ", "done")` = false. REQ com `status: "Done "` é tratada como não-Done → não dispara. Pré-existente à correção. Consistente entre as duas regras (ambas não disparam). Não é regressão.
Severidade: **baixa** — typo, não vetor prático.

**A3 — Assimetria D3: `traceid_orphan_req` aceita link reverso, `req_has_roadmap` exige link direto**
Quando roadmap tem `req_id:` correto mas REQ não tem `roadmap:` campo: `traceid_orphan_req` passa, `req_has_roadmap` dispara. Estado alcançável (ligação unidirecional roadmap → REQ sem back-link na REQ).
Medido: `REQ-done-unquoted.md` + `ROADMAP-has-req-id.md` com `req_id: "REQ-DUNQ"` → `traceid_orphan_req`: silencioso; `req_has_roadmap`: `✗ enforced`.
Pela Regra Dura de Causa Raiz e pelo enunciado da ADR D4 ("as quatro aplicam o mesmo critério"), este sítio pertence a esta REQ como ML adicional — não entra em fila separada.
Severidade: **gap de D4, pertence a esta REQ**.

**A4 — `ref_targets_exist` sem filtro de status**
Dispara para REQ Open com `roadmap:` ghost (arquivo declarado mas inexistente). `req_has_roadmap` e `traceid_orphan_req` não disparam para a mesma REQ (Open → filtrado por D2). As três regras discordam para o mesmo par.
Medido: `REQ-2026-09-29-open-ghost-roadmap.md` → `✗ ref_targets_exist`; silêncio nas outras duas.
A ADR D4 coloca `ref_targets_exist` na mesma tabela das quatro regras que operam sobre o vínculo REQ↔roadmap. A Wave 0 já documentou a divergência. Há duas leituras possíveis: (a) `ref_targets_exist` pergunta "o arquivo declarado existe?" — questão diferente de "deveria ter roadmap?", logo D4 não se aplica a ela; (b) `ref_targets_exist` faz parte do conjunto que D4 nomeia e precisa de status filter. O arquiteto precisa decidir qual e declarar no ADR. Se for (b), a correção pertence a esta REQ.
Severidade: **gap de D4 — necessita decisão do arquiteto sobre o escopo de D4 para `ref_targets_exist`**.

### Confirmação de escopo

`git diff trackfw.yaml`: vazio. Nenhum arquivo do repositório foi modificado por esta auditoria.
