---
status: done
date: 2026-07-30
req: "REQ-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada"
squad: ""
---

# Roadmap: roadmap move sincroniza a referencia da REQ pareada

> Created: 2026-07-30 | Status: done

## Contexto

REQ: `docs/req/REQ-2026-07-30-roadmap-move-sincroniza-a-referencia-da-req-pareada.md`

`trackfw roadmap move` sincroniza pasta e `status:` do roadmap, mas deixa o `roadmap:` da REQ pareada
apontando para o estado anterior — o comando de governança produz um estado que o validador reprova
(`ref_targets_exist`). Constatado quatro vezes em duas sessões consecutivas.

## Critérios de Aceite

- [x] `roadmap move` atualiza o `roadmap:` do frontmatter de toda REQ que aponte para o roadmap movido.
- [x] Linha `Roadmap:` do corpo também atualizada, preservando backticks.
- [x] Descoberta por varredura do `req_dir` casando basename, em layout flat e `by_agent` — cenário
      `roadmap-move-parity/by_agent-discriminant` nos três runtimes.
- [x] Zero REQs → no-op silencioso; múltiplas → todas atualizadas; outra REQ → não tocada — cenários
      `zero-req`, `by_agent-discriminant` e `points-at-other`, todos com vacuity-guard.
- [x] Idempotente: mover duas vezes não altera bytes — cenário `idempotency`, comparando bytes das REQs
      após o segundo move.
- [x] Falha de escrita → diagnóstico nomeando a REQ + exit não-zero, sem desfazer o move — coberto por
      teste unitário nos três runtimes.
- [x] Paridade nos 3 CLIs com cenário byte-a-byte em `make quality` — `scripts/check-roadmap-move-parity.sh`,
      5 cenários, todos comparando Go == Node == Python.
- [x] `make quality` exit 0 (21 cenários de falsificação, 14 gates não-vacuosos) e `validate --json`
      0 violações. Barrier das quatro waves: `passed`.
- [x] **Ordenação lexicográfica por basename** — emenda do contrato durante a Wave 2, após dois dos três
      implementadores reportarem que a ordem estava despinada. Provada nas duas fixtures.
- [x] **Linha `moved` byte-idêntica nos três** — divergência pré-existente do Python corrigida por decisão
      explícita do usuário. `hexdump` confirma `e2 9c 93` (U+2713).

## Mapa de dependências

```
Wave 1 — ML-1A (contrato, orquestrador)
   ↓ barrier — os três runtimes implementam contra o contrato congelado
Wave 2 — ML-2A (Go) ‖ ML-2B (Node.js) ‖ ML-2C (Python)   ← spawn simultâneo, arquivos disjuntos
   ↓ barrier — exige os três concluídos
Wave 3 — ML-3A (auditoria de paridade + não-vacuidade)
```

### Lições dos dois roadmaps anteriores, aplicadas aqui

O ML-1A dos dois roadmaps anteriores falhou **duas vezes pelo mesmo padrão**: pinou a *forma* e deixou o
*comportamento* à interpretação.

| Roadmap | O que foi pinado | O que faltou | Custo |
|---|---|---|---|
| skip de artefato desatualizado | **nomes** dos parâmetros do observador | os **valores** de cada um | 1 wave corretiva, 3 respostas divergentes |
| rótulo de wave com sufixo | os dois **regexes** | **quando** a validação roda | 1 wave corretiva, bug que o teste não pegava |

Regra derivada, aplicada ao ML-1A abaixo: **pinar sempre a ordem das operações e os valores observáveis,
não apenas as estruturas e assinaturas.** Em particular, este ML-1A precisa pinar *quando* a escrita da
REQ ocorre no fluxo do move, *o que* acontece em cada cardinalidade (zero, uma, várias), e *o texto
literal* de cada linha de saída.

---

## Wave 1 — Congelar o contrato (1 ML)
> Dependências: nenhuma

### ML-1A — Pinar o contrato de sincronização da referência
**Status:** ✅ Concluído (contrato autorado pelo orquestrador)
**Agente:** orquestrador (`trackfw_architect`) — autoria exclusiva
**Arquivos afetados:**
- `docs/cli-parity.md` — nova seção sob a governança de referências canônicas

**Seção escrita:** `### roadmap move synchronizes the paired REQ reference`, sob
`## Canonical governance references` em `docs/cli-parity.md`.

**Pinado:**
1. **Direção e momento.** A sincronização é unidirecional (corrige quem aponta para o roadmap) e ocorre
   **após** o rename bem-sucedido, no mesmo ponto onde o `status:` do roadmap já é reescrito.
2. **Fonte de descoberta.** Varredura do `req_dir` — flat **e** `by_agent` — casando o **basename** do
   roadmap no `roadmap:` da REQ. Explicitamente **não** usar o `req:` do roadmap, que é frequentemente
   vazio.
3. **Qual campo é normativo.** Frontmatter é o que o validador lê (`extractRefPath` ignora a forma com
   backticks do corpo); o corpo é atualizado por coerência com o leitor humano, preservando formatação.
4. **Cardinalidade, caso a caso.** Zero → no-op silencioso exit 0. Uma → atualiza. Várias → atualiza
   todas. Aponta para outro roadmap → não toca. Já correta → **nenhuma escrita** (idempotência
   byte-a-byte).
5. **Texto literal de cada linha de saída**, incluindo o caso de falha, e em qual stream sai.
6. **Comportamento em erro:** o move **não** é desfeito; diagnóstico nomeia a REQ; exit não-zero.

**Critérios de aceite:**
- [x] Momento da escrita no fluxo pinado explicitamente — **após** rename bem-sucedido, no mesmo ponto
      onde o `status:` do roadmap já é reescrito, "nunca antes, para que um rename falho não deixe edição
      pendurada".
- [x] Fonte de descoberta pinada, com `by_agent` coberto e o `req:` do roadmap **explicitamente excluído**
      (é `""` recém-criado e slug sem caminho nos existentes).
- [x] Campo normativo vs. campo de coerência distinguidos, com a razão: `extractRefPath` trima aspas mas
      **não** backticks, então a forma do corpo é invisível ao validador — "uma implementação que atualiza
      só o corpo não corrige nada".
- [x] **Cinco** cardinalidades pinadas em tabela (zero, uma, várias, aponta para outro, já correta), não
      quatro como eu havia previsto: separei "aponta para outro roadmap" de "já correta".
- [x] Textos de saída pinados literalmente com stream: `✓ synced <req> → <path>` em stdout;
      `trackfw roadmap move: failed to sync <req>: <cause>` em stderr.
- [x] Comportamento em erro pinado: **não** desfaz o move, tenta as REQs restantes, exit não-zero ao fim
      — "um arquivo não-gravável não esconde os demais".

---

## Wave 2 — Implementar nos três runtimes (3 MLs em paralelo)
> Dependências: ML-1A completo. Arquivos disjuntos — **spawn simultâneo**.

### ML-2A — Go
**Status:** ✅ Concluído (commit `02c5dee`)
**Agente:** Apolo
**Arquivos afetados:** `internal/generators/roadmap.go` (`MoveRoadmap`, linha ~326, após o
`rewriteRoadmapStatus` da linha ~372), testes correspondentes

**Evidência de implementação:**
- `syncREQReferences`, `scanREQFiles`, `extractFrontmatterRoadmap`, `rewriteREQRoadmapRef` adicionadas em `internal/generators/roadmap.go`
- 10 testes novos em `internal/generators/roadmap_test.go` (9 unitários + 1 integração)
- `go build ./...` ✓ | `go test ./...` 15 pacotes ok | `go vet ./...` ✓

**Divergência relatada (contrato incompleto):** A spec do ML diz inserir "antes do `appendTransitionLog`",
mas o contrato `cli-parity.md` pina que `✓ synced` vem APÓS `✓ moved`. O `fmt.Printf("✓ moved ...")` está
APÓS `appendTransitionLog` no código. Inserção correta: após o `fmt.Printf("✓ moved ...")`, antes do
`return nil`. Contrato é a autoridade; spec do ML estava inconsistente com ele.

**Critérios de aceite:**
- [x] Todas as cardinalidades conforme o contrato. (zero, uma, várias, outro roadmap, já correta)
- [x] Idempotência provada por comparação de bytes após dois moves. (`TestSyncREQ_Idempotency_ByteLevel`)
- [x] `by_agent` coberto por teste. (`TestSyncREQ_ByAgent`)
- [x] `go build ./...`, `go test ./...`, `go vet ./...` passam.

### ML-2B — Node.js
**Status:** ✅ Concluído
**Agente:** Apolo
**Commit:** `ba13af9`
**Arquivos afetados:** `npm/src/generators/roadmap.js` (`moveRoadmap`), `npm/tests/roadmap_move.test.js`

**Critérios de aceite:**
- [x] Comportamento e textos equivalentes ao Go (5 cardinalidades + output pinado literalmente).
  **Evidência:** 9 testes dedicados ao syncReqReferences cobrindo zero/uma/várias/outro/já-correta.
- [x] Idempotência provada por comparação de bytes após dois moves.
  **Evidência:** teste "idempotência byte-a-byte: mover duas vezes não altera bytes da REQ" + teste "referência já correta: nenhuma escrita" usando `Buffer.equals`.
- [x] `by_agent` coberto por teste.
  **Evidência:** teste "REQ em req_dir/<agente>/<estado>/ é encontrada e reescrita" — REQ em `req_dir/zeus/wip/` localizada e sincronizada.
- [x] Formatação do corpo (backticks) preservada.
  **Evidência:** teste "backticks no corpo são preservados após reescrita" — `Roadmap: \`${newPath}\`` verificado byte-exato.
- [x] `cd npm && npm test` passa.
  **Evidência:** 339 testes, 0 falhas.

**Divergência reportada ao orquestrador:** a ordem de varredura de múltiplas REQs não está pinada no contrato ("na ordem de varredura"). `resolveReqFiles` em flat usa `fs.readdirSync` sem sort, que não garante ordem lexicográfica. O teste de várias REQs asserta o **conjunto** (não sequência). Se o Go ordenar e o Node não, ML-3A detectará divergência de paridade. Recomendo ao orquestrador pinar explicitamente se a ordem é intencional.

### ML-2C — Python
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:**
- `pypi/trackfw/generators/roadmap.py` — helpers `_get_frontmatter_roadmap_value`, `_rewrite_req_roadmap_ref` e `sync_paired_req_references`
- `pypi/trackfw/commands/roadmap.py` — `_cmd_move` chama `sync_paired_req_references` e imprime saída pinada
- `pypi/tests/test_generators_roadmap.py` — 21 novos testes (5 cardinalidades + idempotência + by_agent + backticks)

**Critérios de aceite:**
- [x] Comportamento e textos equivalentes ao Go e Node.
  **Evidência:** todas as 5 cardinalidades testadas e passando; saída `✓ synced` (U+2713 + U+2192) e `trackfw roadmap move: failed to sync` implementadas conforme contrato. `_cmd_move` imprime após o move bem-sucedido.
- [x] Idempotência provada por comparação de bytes após dois moves.
  **Evidência:** `test_idempotencia_byte_a_byte_duas_chamadas` — segunda chamada retorna `synced=[]` e bytes do arquivo REQ são idênticos.
- [x] `by_agent` coberto por teste.
  **Evidência:** `test_by_agent_req_encontrada` — REQ em `req_dir/zeus/wip/` localizada e sincronizada via `resolve_req_files`.
- [x] Formatação do corpo (backticks) preservada.
  **Evidência:** `test_backticks_preservados_no_corpo` — `Roadmap: \`{new_path}\`` verificado literalmente.
- [x] Suíte Python passa.
  **Evidência:** `cd pypi && python3 -m pytest` → 723 passed, 0 failed (21 testes novos adicionados ao total de 701 anteriores).

**Divergência reportada ao orquestrador:** Python ordena a lista de REQs (`sorted(resolve_req_files(cfg))`). Node.js não ordena (`readdirSync` sem sort). Se o Go também não ordena, o ML-3A detectará divergência de paridade na cardinalidade "várias REQs". Recomendo pinar explicitamente se a ordem é determinística.

---

## Wave 3 — Convergir ordenação e a linha `moved` (3 MLs em paralelo, corretivo)
> Dependências: Wave 2 completa. Emenda do contrato feita. Um ML por runtime — **spawn simultâneo**.

**Origem:** auditoria do orquestrador **executando os três CLIs**, com fixture `by_agent` construída
para discriminar ordenação por caminho de ordenação por basename. As três suítes estavam verdes
(339 · 723 · Go limpo) e mesmo assim havia duas divergências.

### Divergência 1 — ordenação: nenhum dos três cumpre o contrato

Dois dos três implementadores **reportaram** que a ordem não estava pinada — e estavam certos: o
contrato dizia "na ordem de varredura", que não é uma ordem. Emendado para **lexicográfica por
basename da REQ**. Medido depois da emenda:

| Fixture | Go | Node.js | Python |
|---|---|---|---|
| flat, 3 REQs | ordenado | ordenado | ordenado |
| `by_agent`, `apolo/REQ-aaa` + `zeus/REQ-zzz` | `zzz, aaa` ❌ | `zzz, aaa` ❌ | `aaa, zzz` ✅ |
| `by_agent`, `apolo/REQ-zzz` + `zeus/REQ-aaa` | — | — | `zzz, aaa` ❌ |

**Os três estão errados, e cada um por um motivo diferente:**
- **Go** — `filepath.Glob` ordena dentro de cada padrão, mas `scanREQFiles` concatena por agente e por
  estado, e a lista de estados é fixa (`backlog, analyzing, wip, blocked, done, abandoned`), que nem é
  lexicográfica.
- **Node.js** — `fs.readdirSync` sem `.sort()`. Concordava em flat **por acidente do APFS**; não há
  garantia entre sistemas de arquivos.
- **Python** — `sorted()` sobre a lista de **caminhos completos**, não de basenames. Passou na primeira
  fixture por coincidência (`apolo/…aaa` < `zeus/…zzz`) e falha na fixture discriminante.

Lição: "determinístico" não é "conforme". Python era determinístico e ainda assim divergente do
contrato.

### Divergência 2 — linha `moved` divergente desde antes desta REQ

| Runtime | Saída em `origin/main` |
|---|---|
| Go | `✓ moved <basename> → <targetDir>` |
| Node.js | `✓ moved <basename> → <targetDir>` |
| Python | `Roadmap movido para: <caminho completo>` |

Três diferenças: idioma, forma e conteúdo. **Pré-existente**, verificado em
`git show origin/main:pypi/trackfw/commands/roadmap.py`. Fora do escopo original desta REQ, incluída
por decisão explícita do usuário: a regra dura de paridade do projeto se aplica, e a auditoria
byte-a-byte da própria feature não passa com a linha anterior divergindo.

**Mudança observável:** o CLI Python deixa de imprimir `Roadmap movido para: <path>`.

### ML-2D — Go: ordenar por basename
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `internal/generators/roadmap.go` (`syncREQReferences`), `internal/generators/roadmap_test.go`

**Implementação:** `sort.Slice` por basename (desempate por caminho completo) inserido em `syncREQReferences` logo após `scanREQFiles`, antes do loop de varredura. Import `"sort"` adicionado.

**Evidência — fixture discriminante (`by_agent`, agents: [zeus, apolo]):**
```
docs/req/apolo/done/REQ-zzz.md  → aponta para ROADMAP-order.md
docs/req/zeus/backlog/REQ-aaa.md → aponta para ROADMAP-order.md
```
Saída observada (TestSyncREQ_ByAgent_OrderByBasename PASS):
```
✓ synced REQ-aaa.md → docs/roadmaps/zeus/wip/ROADMAP-order.md   ← linha 0
✓ synced REQ-zzz.md → docs/roadmaps/zeus/wip/ROADMAP-order.md   ← linha 1
```
Ordem por caminho teria produzido `zzz, aaa` (errado). Ordem por basename produz `aaa, zzz` (correto).

**Critérios de aceite:**
- [x] Ordenação lexicográfica por basename, aplicada à lista final combinada.
- [x] Teste com fixture `by_agent` discriminante (`apolo/REQ-zzz` + `zeus/REQ-aaa` → `aaa, zzz`). (`TestSyncREQ_ByAgent_OrderByBasename` — asserta sequência exata)
- [x] `go build ./...`, `go test ./...`, `go vet ./...` passam. (15 pacotes ok)

### ML-2E — Node.js: ordenar por basename
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `npm/src/generators/roadmap.js` (`syncReqReferences`), testes em `npm/tests/`

**Critérios de aceite:**
- [x] Ordenação **explícita** por basename — não confiar na ordem do `readdirSync`.
- [x] Teste com fixture `by_agent` discriminante.
- [x] Teste de múltiplas REQs asserta a **sequência**, não apenas o conjunto.
- [x] `cd npm && npm test` passa.

**Evidência da fixture discriminante:**
```
✓ syncReqReferences — by_agent discriminante: ordenação por basename, não por caminho completo
25 testes — 25 passaram, 0 falharam
ℹ pass 339 (suite completa)
```
agents=[zeus,apolo]; apolo/done/REQ-zzz.md + zeus/backlog/REQ-aaa.md: aaa emitido antes de zzz ✓

### ML-2F — Python: ordenar por basename e alinhar a linha `moved`
**Status:** ✅ Concluído
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/generators/roadmap.py` (~476), `pypi/trackfw/commands/roadmap.py`
(~106), testes em `pypi/tests/`

**Critérios de aceite:**
- [x] Ordenação por **basename**, não por caminho completo.
- [x] Teste com fixture `by_agent` discriminante.
- [x] `✓ moved <basename> → <targetDir>` byte-idêntico ao Go, com U+2713 e U+2192.
- [x] Suíte Python passa.

**Evidências:**
- Fixture discriminante `by_agent` (`apolo/done/REQ-zzz` + `zeus/backlog/REQ-aaa`) → `synced = ["REQ-aaa.md", "REQ-zzz.md"]` ✓
- Comparação lado a lado Go vs Python:
  - Go: `✓ moved ROADMAP-2026-07-29-test-move.md → docs/roadmaps/wip`
  - Python: `✓ moved ROADMAP-2026-07-29-test-move.md → docs/roadmaps/wip`
- Suíte completa: 724 passed, 0 failed.

**Disjunção:** um ML por runtime, arquivos sem interseção. Paralelizáveis.

## Wave 4 — Auditoria de paridade (1 ML)
> Dependências: **barrier** — Waves 2 e 3 completas (ML-2A a ML-2F).

### ML-3A — Auditar paridade e provar não-vacuidade
**Status:** ✅ Concluído
**Agente:** Artemis
**Commit:** `1bbc8b6`

**Ações executadas:**
1. `scripts/check-roadmap-move-parity.sh` criado: executa os três runtimes em fixtures isoladas
   (cp-r por runtime para evitar conflito de filesystem), compara stdout byte-a-byte nas
   **cinco** cardinalidades (contrato tem cinco — ML-3A dizia quatro, mas `cli-parity.md` pina cinco;
   implementado conforme o contrato, divergência reportada aqui).
2. Seam de falsificação: Cenário 20 em `check-gates-falsify.sh` — sed corrompe o comparador de sort
   do Node.js (`path.basename(a)` → `a`), gate detecta divergência na fixture discriminante.
   Seam verificado manualmente: exit 1, diagnóstico `roadmap-move-parity/by_agent-discriminant/node`.
3. Fixture discriminante `by_agent` (`apolo/done/REQ-zzz` + `zeus/backlog/REQ-aaa`) asserta
   sequência posicional (linha 0 = aaa, linha 1 = zzz). Coincident fixture excluída intencionalmente.
4. Gate encadeado no `parity` do Makefile antes do `check-gates-falsify.sh`.
   `GATES_MUTATION_CHECK` atualizado (Cenário 18). Contador: 21 cenários / 14 gates.

**Evidências de aceite:**
```
make quality exit 0
validate --json: 0 violations
git status: limpo após commit

Go: 15 pacotes ok
Node.js: 339 pass, 0 fail
Python: 724 passed

Seam ativo (Node.js corrompido):
  FAIL [roadmap-move-parity/by_agent-discriminant/node]: line 0 must be REQ-aaa.md
  (basename order); got: [✓ synced REQ-zzz.md → docs/roadmaps/zeus/wip/...]; exit 1

Gate limpo:
  OK [roadmap-move-parity/zero-req]
  OK [roadmap-move-parity/one-req]
  OK [roadmap-move-parity/by_agent-discriminant]
  OK [roadmap-move-parity/points-at-other]
  OK [roadmap-move-parity/idempotency]
  Falsification: OK [falsify/roadmap-move-parity/discriminant-wrong-order-not-detected]
```

**Critérios de aceite:**
- [x] Cinco cardinalidades cobertas nos três runtimes, byte-a-byte. (contrato tem 5, não 4)
- [x] Vacuity-guard presente; seam de falsificação prova poder de reprovação.
- [x] Fixture `by_agent` discriminante assertando sequência posicional.
- [x] Linha `moved` comparada indiretamente via stdout byte-a-byte em todos os cenários (flat + by_agent).
- [x] Idempotência: dois moves, bytes da REQ inalterados.
- [x] Seam verificado por execução: exit 1 com diagnóstico exato.
- [x] `make quality` exit 0, `validate --json` 0 violações, `git status` limpo.

---

## Matriz de verificação empírica do orquestrador (Wave 3)

Executando os três CLIs, não lendo relatórios.

**Ordenação — as duas fixtures `by_agent`, `agents: [zeus, apolo]`:**

| Fixture | Esperado | Go | Node.js | Python |
|---|---|---|---|---|
| Coincidente (`apolo/REQ-aaa` + `zeus/REQ-zzz`) | `aaa, zzz` | `aaa, zzz` | `aaa, zzz` | `aaa, zzz` |
| **Discriminante** (`apolo/REQ-zzz` + `zeus/REQ-aaa`) | `aaa, zzz` | `aaa, zzz` | `aaa, zzz` | `aaa, zzz` |

A fixture coincidente é a que **não prova nada** — foi nela que o Python passou por acidente na Wave 2,
porque `apolo/…aaa` < `zeus/…zzz` casa com a ordem de basename. A discriminante inverte os basenames
entre os agentes e separa os dois critérios. Qualquer gate desta feature **precisa** usar a
discriminante, ou nasce vacuoso.

**Linha `moved` — byte-a-byte:**

```
go   : ✓ moved ROADMAP-2026-01-01-t.md → docs/roadmaps/wip
node : ✓ moved ROADMAP-2026-01-01-t.md → docs/roadmaps/wip
py   : ✓ moved ROADMAP-2026-01-01-t.md → docs/roadmaps/wip
```

`diff` go↔node e go↔py sem diferenças. `hexdump` confirma `e2 9c 93` (U+2713) no início.

**Suítes:** `go build`/`test`/`vet` limpos · `npm test` 339 passed · `pytest` **724 passed** ·
`make quality` exit 0 (20 cenários de falsificação, 13 gates não-vacuosos) · árvore limpa.
