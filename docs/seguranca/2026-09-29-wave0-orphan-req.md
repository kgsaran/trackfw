# Wave 0 — Threat Model: `traceid_orphan_req` reprova estado correto

> Data: 2026-09-29 | Autor: hades-tf | REQ: REQ-2026-09-29 | Branch: fix/orphan-req-reprova-estado-correto

---

## 1. Enumeration completeness

### Lista de regras que decidem "esta REQ deveria ter roadmap?"

A ADR D4 declara que duas regras devem concordar: `req_has_roadmap` e `traceid_orphan_req`. A varredura completa de todos os `applyRule*(` em `internal/validator/validator.go` e `internal/validator/validator_traceid.go` encontrou **quatro** regras que operam sobre o vínculo REQ↔roadmap. A lista está **fechada** — qualquer regra que opine sobre o assunto passa por `applyRule*` com seu nome.

| regra | critério exato | severidade padrão | isenções | filtro de status da REQ |
|---|---|---|---|---|
| `req_has_roadmap` | REQ sem `roadmap:` apontando para caminho `.md` real | error | grandfathering por data < 2026-09-03 → warning | **nenhum** — varre todas as REQs |
| `traceid_orphan_req` | REQ com `req_id` cujo valor não existe como `trace_id_field` em nenhum roadmap | error | nenhuma | **nenhum** — só ativa quando `trace_id_field` está configurado |
| `ref_targets_exist` | REQ com `Roadmap:` apontando para arquivo que não existe (ou stale) | warning | escopo limitado: wip, blocked, backlog, analyzing (done/abandoned excluídos, declarado em docs/cli-parity.md) | **nenhum** — varre todas as REQs |
| `req_roadmap_lifecycle` | REQ Open com roadmap pareado em `done/` | warning (promovível) | nenhuma | **tem**: só dispara se `reqStatusIsOpen` retornar verdadeiro |

**ADR D4 está subespecificada.** Declara que duas regras devem concordar e há quatro. As duas ausentes participam da mesma superfície e produzem vereditos divergentes, descritos abaixo.

### Onde as regras discordam sobre o mesmo fato — medido em fixture

Fixture em scratchpad com `trace_id_field: req_id` configurado. Cenário `REQ-2026-09-29-open-with-roadmap.md` (par válido: REQ tem `req_id: "REQ-002"`, roadmap tem `req: "docs/roadmaps/wip/ROADMAP-2026-09-29-paired.md"`, mas o roadmap não tem `req_id`):

```
req_has_roadmap:        PASS  ← vê o campo roadmap: com caminho .md real
traceid_orphan_req:     FAIL  ← não encontra req_id="REQ-002" em nenhum roadmap

saída literal:
✗ traceid_orphan_req: req "REQ-2026-09-29-open-with-roadmap.md" has req_id="REQ-002" but no Roadmap with same id
```

Cenário `REQ-2026-09-29-ghost-path.md` (roadmap: aponta para arquivo inexistente):

```
req_has_roadmap:     PASS  ← extractRefPath aceita qualquer caminho .md, nunca faz os.Stat
ref_targets_exist:   FAIL  ← verifica a existência do arquivo
traceid_orphan_req:  FAIL  ← nenhum roadmap com req_id correspondente

saída literal:
✗ req "REQ-2026-09-29-ghost-path.md" links to Roadmap "docs/roadmaps/wip/ROADMAP-GHOST-NONEXISTENT.md" which does not exist
✗ traceid_orphan_req: req "REQ-2026-09-29-ghost-path.md" has req_id="REQ-2026-09-29-ghost-path" but no Roadmap with same id
```

**Resultado da enumeração:** A lista está fechada em quatro. Duas delas (`req_has_roadmap` e `traceid_orphan_req`) não têm filtro de status e produzem C1. `traceid_orphan_req` produz C2. `ref_targets_exist` opera numa superfície adjacente (existência do arquivo, não o vínculo de ID). `req_roadmap_lifecycle` já tem filtro de status e está fora da falha descrita.

---

## 2. Threat model

O adversário aqui é o implementador apressado e o arquiteto otimista, não um atacante externo.

**Vetor A — Corrigir C1 sem corrigir C2**

D2 diz que o sinal legítimo é "REQ `Done` sem roadmap". A correção óbvia é adicionar um filtro `reqStatusIsDone` em `req_has_roadmap` e `traceid_orphan_req`. Isso fecha C1 (REQs `Open` sem roadmap deixam de disparar). Mas C2 continua viva: o par válido (`REQ` com `req_id`, roadmap com `req:` mas sem `req_id`) ainda dispara `traceid_orphan_req`. O implementador ve a regra continuar falhando e conclui que a correção não funcionou — exatamente como o reportante avisou. O parecer atual da ADR é que "as duas se parecem no log e diferem no mecanismo" e isso é verdade; o risco é que a sprint fecha com D2 implementado e D3 marcado como "pendente para próxima sprint", entrando na fila das 36 REQs abertas.

**Vetor B — Implementar D3 como correspondência por `req:` sem tratar stale/basename/Windows**

D3 diz: casar também pelo campo `req:` do roadmap. O implementador faz uma comparação de string simples entre o valor de `req:` e o caminho da REQ. Isso fecha C2 para pares recentes gerados por `roadmap new`. Silenciosamente FALHA para:
- 17 roadmaps com `req:` apontando para `docs/requisições/` (caminho migrado)
- 7 roadmaps com `req:` sem extensão `.md` (IDs puros — rejeitados por `extractRefPath`)
- 8 roadmaps com `req:` vazio/null
- 18 roadmaps sem campo `req:` algum
- Roadmaps gerados no Windows com `req: "docs\\req\\REQ-..."` (backslash literal)

Para os pares stale e os pares sem `req:`, `traceid_orphan_req` continua disparando mesmo após D3 implementado — e o operador não sabe se o falso-positivo que vê é "stale" ou "genuinamente órfã". O controle degrada para ruído.

**Vetor C — O cutoff tornar-se silenciosamente redundante (ou não)**

A ADR Consequences diz: "O cutoff temporal de `req_has_roadmap` pode tornar-se redundante sob D1/D2." A medição desta wave refuta isso. Dos 13 grandfathered atuais, **3 são `Done`** (os 10 restantes são `Superseded`). Sob D2, uma REQ `Done` sem roadmap dispara. Sem o cutoff, essas 3 REQs pré-legado voltam a ser violations. O implementador que remove o cutoff por interpretação da frase "pode tornar-se redundante" introduz 3 novas violations no CI sem ter mudado nenhum artefato real — o efeito do "número que muda sozinho entre versões" que as Consequences citam como algo que entra nas release notes.

**Vetor D — Tratamento silencioso de `Superseded`, `Closed`, `done` (lowercase)**

D2 diz "REQ `Done`". O código existente (`reqStatusIsDone`) usa `strings.EqualFold(val, "done")` — trata `done` lowercase corretamente. Mas `Superseded` (14 REQs no corpus) e `Closed` (2 REQs) não são `Done` por esse comparador. Se o implementador não decidir explicitamente como tratá-los, o comportamento padrão depende do predicado escolhido: `reqStatusIsDone` retorna `false` para `Superseded`, logo essas REQs sem roadmap deixariam de disparar sob D2 — sem que isso seja uma decisão consciente. O corpus tem 14 `Superseded` + 2 `Closed` potencialmente afetados.

**Vetor E — D3 via correspondência por `req:` herda o bug de namespace cruzado (AC8 REQ-2026-09-28)**

AC8 de REQ-2026-09-28 documenta que `FindRoadmapLinkingREQ` atravessa fronteiras de agente: o roadmap do agente A pode ter `req:` apontando para a REQ do agente B. Se D3 usa o campo `req:` para satisfazer `traceid_orphan_req`, um roadmap de outro agente pode "parar" a violação de uma REQ genuinamente órfã. A REQ fica sem roadmap real e o controle reporta silenciosamente "tudo bem". AC8 está aberto e sem REQ de correção — o risco herda diretamente para D3.

**Vetor F — REQ-2026-09-25 como REQ duplicada em aberto**

`docs/req/REQ-2026-09-25-regra-de-rastreabilidade-ignora-o-estado-da-req-e-acusa-backlog-como-orfao.md` está `Open` e endereça o mesmo issue #435 com roadmap em `backlog/`. A Regra Dura pede mesma causa = mesma REQ. Com duas REQs abertas para a mesma causa, um ML futuro pode ser despachado contra a REQ antiga sem conhecer a mais completa.

---

## 3. Falsification targets in both directions

### Superfície 1: `req_has_roadmap` — filtro de status (D1/D2)

| Direção | onde a sabotagem entra | gate que deveria capturar |
|---|---|---|
| **Falso negativo** (filtro fraco demais): REQ `Done` sem roadmap não dispara | Predicado errado ou invertido (`isOpen` em vez de `isDone`) | Teste de fixture: REQ `Done` sem `roadmap:` deve disparar `req_has_roadmap` |
| **Falso positivo** (filtro estrito demais): REQ `Open` sem roadmap ainda dispara | `reqStatusIsDone` retorna falso-positivo para `Open` | Teste de fixture: REQ `Open` sem `roadmap:` NÃO deve disparar `req_has_roadmap` — e um teste deve falhar se a decisão for invertida por diretório |

**Zona cinzenta declarada:** `Superseded`, `Closed`, `done` (lowercase). `reqStatusIsDone` trata `done` corretamente (EqualFold). Para `Superseded` e `Closed`: o implementador deve decidir explicitamente e documentar no código — fail-open (não dispara, como `Open`) ou fail-closed (dispara, como `Done`). A fail-closed tem precedente na regra de data do cutoff. A decisão deve ser escrita; não pode emergir implicitamente da ausência de um `if`.

### Superfície 2: `traceid_orphan_req` — correspondência por `req:` (D3)

| Direção | onde a sabotagem entra | gate que deveria capturar |
|---|---|---|
| **Falso negativo** (D3 muito permissivo): REQ genuinamente órfã passa porque um roadmap de outro agente tem `req:` apontando para ela | AC8 de REQ-2026-09-28 não corrigido; D3 herda o bug | Teste de fixture: roadmap de agente A com `req:` → REQ de agente B não deve satisfazer `traceid_orphan_req` da REQ de B quando B tem roadmap próprio sem `req:` |
| **Falso positivo** (D3 muito restrito): par válido não é reconhecido porque `req:` tem backslash (Windows) | Implementação não chama `normalizeRefSeparator` | Medido na fixture: `req: "docs\\req\\REQ-x.md"` — `traceid_orphan_req` ainda dispara. `normalizeRefSeparator` já existe em `validator.go:3285`; D3 deve usá-lo |
| **Falso positivo** (D3 muito restrito): par válido não é reconhecido porque `req:` não termina em `.md` | Valor é ID puro, sem extensão | `extractRefPath` rejeita silenciosamente (requer `.md`); 7 roadmaps nesta categoria — par não seria satisfeito mesmo com D3 |
| **Falso negativo** (stale satisfaz D3): roadmap com `req:` stale aponta para REQ X, mas REQ X mudou de nome — REQ Y com mesmo basename em outro lugar é matchada | Basename fallback sem verificação de unicidade | Medição de basename: todos os 237 REQs têm basenames únicos (medido pelo arquiteto em 2026-09-28: "basenames únicos 233, colisões 0") — risco baixo hoje, mas não declarado |

**Discriminador que o implementador deve rodar no corpus do consumidor antes de declarar D3 completo:**

```bash
find <roadmap_dir> -name "*.md" | xargs grep "^req:" | grep -v '<req_dir>/'
```

Qualquer saída indica `req:` stale ou de formato não-padrão. Para cada linha: se o REQ pareado tem `req_id`, o par ainda vai disparar `traceid_orphan_req` após D3, porque o `req:` não resolve. O número de baseline entries que D3 fecha vs. não fecha depende dessa contagem no corpus do consumidor — não verificável a partir deste repositório.

### Superfície 3: `ref_targets_exist` — divergência com `req_has_roadmap`

| Direção | onde a sabotagem entra | gate |
|---|---|---|
| **Falso negativo**: REQ com caminho ghost (`roadmap: "GHOST.md"`) não é sinalizada | `req_has_roadmap` aceita qualquer `.md` sem `os.Stat`; `ref_targets_exist` faz o stat mas está fora do escopo D4 | Medido: `req_has_roadmap` PASSA para `GHOST.md`; `ref_targets_exist` DISPARA. ADR D4 deve nomear os 4 controles. |
| **Falso positivo**: REQ com roadmap real em `done/` reportada como "not exist" porque `ref_targets_exist` só varre estado pelo caminho literal | `resolveRoadmapRefStatus` já faz basename fallback — stale é classificado, não "not exist" | Coberto pelo ML-1A de REQ-2026-09-28 (Done) |

---

## 4. Declared residual

O que esta análise aceita não cobrir:

**R1 — O corpus do consumidor não é acessível.** As 14 entradas congeladas no `.trackfw-baseline.json` do consumidor podem ou não ser pares com `req:` válido. A medição de D3 aqui é sobre o corpus deste repositório (234 roadmaps). A discriminação exata para o consumidor requer o comando da Superfície 2 acima rodado no repo do consumidor.

**R2 — O cutoff não cobre REQs `Done` legadas sem roadmap no corpus externo.** No corpus interno, 3 das 13 grandfathered são `Done`. Num consumidor com REQs `Done` pré-2026-09-03 sem roadmap, a combinação D2 (só dispara para `Done`) + ausência de cutoff voltaria a cobrar essas REQs. O cutoff atual (`reqRoadmapCutoff = "2026-09-03"`) isenta o corpus DESTE repositório; repositórios consumidores com datas diferentes precisam de cutoff configurável ou de um mecanismo distinto. Não está no escopo desta REQ.

**R3 — AC8 de REQ-2026-09-28 (namespace cruzado no `FindRoadmapLinkingREQ`) está aberto.** Afeta D3 diretamente (Vetor E acima). A correção de AC8 é pré-condição para que D3 seja safe em repositórios com `roadmap_namespacing: by_agent`. Esta REQ não endereça AC8 — precisa ser tratado na mesma REQ ou como bloqueador explícito.

**R4 — `Superseded` e `Closed` carecem de definição explícita.** Há 16 REQs nessa condição no corpus interno. Nenhuma das 4 regras enumera comportamento para esses valores. O comportamento emerge do predicado de status sem ser decidido explicitamente. Não é escopo desta wave — é escopo do ML-1A.

**R5 — Duas REQs abertas para a mesma causa (#435).** REQ-2026-09-25 e REQ-2026-09-29 abordam o mesmo issue. A Regra Dura manda absorver, mas a decisão é do arquiteto, não desta wave.

---

## Corpus delta — Task 3

Medido em `docs/req/` (237 arquivos), rodando `./bin/trackfw validate` na raiz do repositório (sem `trace_id_field` configurado — `traceid_orphan_req` é INERTE neste repo):

**Estado atual (`req_has_roadmap`):**
- 0 violations (enforced)
- 13 warnings (grandfathered, pre-cutoff 2026-09-03, sem roadmap)
  - 3 com `status: Done`
  - 10 com `status: Superseded`

**Saída literal:**
```
⚠  req_has_roadmap grandfathering: 13 REQ(s) without a linked Roadmap exempt as created before the cutoff 2026-09-03, 0 enforced as created on/after it, 237 REQ(s) scanned
```

**Sob D1/D2 (apenas REQ `Done` dispara):**
- 9 REQs `Done` pós-cutoff sem roadmap → continuam como violations (sinal legítimo preservado)
- 15 REQs `status: done` (lowercase) → tratadas como `Done` pelo `reqStatusIsDone` EqualFold → podem disparar se sem roadmap (medição pendente — grep não substitui binary)
- 13 grandfathered: os 3 `Done` precisam do cutoff para continuar supressos; sem cutoff virariam violations
- 14 `Superseded` + 2 `Closed` → comportamento não definido por D2 (ver R4)

**Refutação da Consequences da ADR:** "O cutoff pode tornar-se redundante sob D1/D2" — REFUTADO. Dos 13 grandfathered, 3 são `Done`. Sob D2, esses 3 voltariam a dispara se o cutoff fosse removido. O cutoff é necessário para a população `Done` legada pré-padronização.

---

## Apêndice: fixture usada

Localização: `/private/tmp/claude-501/.../scratchpad/fixture-wave0/`
`trackfw.yaml`: `trace_id_field: req_id`, `req_dir: docs/req`, `roadmap_dir: docs/roadmaps`

Cenários testados:
1. `REQ-2026-09-29-open-no-roadmap.md` — Open, `req_id: "REQ-001"`, sem roadmap → C1
2. `REQ-2026-09-29-open-with-roadmap.md` — Open, `req_id: "REQ-002"`, roadmap com `req:` mas sem `req_id` → C2
3. `REQ-2026-09-29-done-no-roadmap.md` — Done, `req_id: "REQ-003"`, sem roadmap → sinal legítimo
4. `REQ-2026-09-29-ghost-path.md` — Open, `roadmap:` apontando para arquivo inexistente → divergência `req_has_roadmap` vs `ref_targets_exist`
5. `REQ-2026-09-29-windows-sep.md` — Open, roadmap com `req: "docs\\req\\..."` → separador Windows

Todos confirmados com `./bin/trackfw validate` rodando na fixture. Saídas literais nas seções acima.
