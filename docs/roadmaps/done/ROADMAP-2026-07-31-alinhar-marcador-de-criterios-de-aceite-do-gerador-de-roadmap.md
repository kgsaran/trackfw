---
status: done
date: 2026-07-31
req: "docs/req/REQ-2026-07-31-alinhar-marcador-de-criterios-de-aceite-gerado-por-roadmap-new-com-o-validator.md"
squad: ""
---

# Roadmap: Alinhar marcador de criterios de aceite do gerador de roadmap

> Created: 2026-07-31 | Status: done

## Context

REQ: docs/req/REQ-2026-07-31-alinhar-marcador-de-criterios-de-aceite-gerado-por-roadmap-new-com-o-validator.md
ADR: docs/adr/ADR-2026-07-31-gerador-de-roadmap-emite-heading-consolidado-de-criterios-de-aceite.md

`trackfw roadmap new` gera um roadmap que `trackfw validate` rejeita assim que ele entra em `wip`:

```
✗ roadmap "ROADMAP-....md" is in wip but has no acceptance criteria block
```

O gerador emite `**Acceptance criteria:**` (negrito, por ML); o validador exige o heading
`## Acceptance Criteria` ou `## Critérios de Aceite`
(`internal/config/config.go:83` → `internal/validator/validator.go:989`).

Contornado manualmente em **três ciclos consecutivos**. O gerador de REQ
(`internal/generators/req.go:93`) já está correto — a divergência é só no de roadmap.

### Bloco exato a inserir (byte-a-byte idêntico nos 3 CLIs)

Posicionado **após a seção `## Context`** (após a linha `REQ: ...`, e após `ADR: ...` quando
houver) e **antes de `## Wave 1`**:

```
## Acceptance Criteria
<!-- Consolidated criteria for this roadmap. Detail per ML in the waves below. -->
- [ ]
- [ ]
```

Seguido de uma linha em branco antes de `## Wave 1`. **Sem espaço à direita** em nenhuma linha —
`- [ ]` termina no `]`.

Convenção espelhada de `internal/generators/req.go:93`, que usa `## Acceptance Criteria` seguido
do conteúdo e linha em branco.

### Os dois caminhos de geração

Ambos precisam do bloco:

| Caminho | Go | Node | Python |
|---|---|---|---|
| Template simples | `internal/generators/roadmap.go` ~93-118 | `npm/src/generators/roadmap.js` ~444 | `pypi/trackfw/generators/roadmap.py` ~211 |
| `--from-req` | `internal/generators/roadmap.go` ~148-183 | `npm/src/generators/roadmap.js` ~500 | `pypi/trackfw/generators/roadmap.py` ~321 |

No caminho `--from-req` do Go o corpo é montado em `body := fmt.Sprintf(...)` com `mlSection` ao
final; o bloco entra entre `REQ: %s%s` (o segundo `%s` é o `adrRef`) e `mlSection`.

### Testes que podem quebrar

Referenciam o template gerado e devem ser atualizados no mesmo ML:

- Go: `internal/generators/scaffold_test.go`, `internal/validator/validator_test.go`,
  `internal/commands/barrier_test.go`, `internal/commands/barrier_contract_test.go`,
  `internal/serve/api_board_test.go`
- Node: `npm/tests/barrier.test.js`, `npm/tests/barrier-contract.test.js`, `npm/tests/init.test.js`
- Python: `pypi/tests/test_generators_roadmap.py`, `pypi/tests/test_barrier.py`,
  `pypi/tests/test_barrier_contract.py`, `pypi/tests/test_generators_init.py`

### Dependências e paralelismo

**Wave 1 tem paralelismo real** — os três geradores são arquivos disjuntos, e cada ML mexe apenas
no seu CLI e nos testes daquele CLI. Nenhum arquivo compartilhado, nem índice do git (os agentes
entregam uncommitted; Zeus commita).

**Importante:** `make parity` e `make quality` vão **falhar** enquanto os três não estiverem
prontos, porque `scripts/check-artifact-parity.sh` compara os artefatos gerados byte-a-byte entre
os CLIs. Por isso **nenhum ML da Wave 1 tem `make parity` nos critérios de aceite** — cada um
valida só o próprio CLI. A paridade é a **Wave 2**, que funciona como barreira.

## Critérios de Aceite

Consolidados da REQ (AC1–AC7). Detalhamento por microlote nas waves abaixo.

- [x] Roadmap gerado e movido para `wip` passa em `validate` sem edição manual, nos 3 CLIs
- [x] Heading consolidado emitido **sem remover** os blocos `**Acceptance criteria:**` por ML
- [x] Vale também para `roadmap new --from-req`
- [x] Os três geradores produzem artefato byte-a-byte idêntico
- [x] Roadmaps existentes não são invalidados — `validate` segue verde no repositório
- [x] Seam de falsificação prova que o ciclo falha se o gerador voltar ao marcador antigo
- [x] `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes

---

## Wave 1 — Geradores (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos — rodam simultaneamente.

### ML-1A — Gerador Go
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `internal/generators/roadmap.go` + testes Go afetados

**Ações:** inserir o bloco exato nos **dois** caminhos (template simples e `--from-req`);
atualizar os testes Go que asseguram o template.

**Acceptance criteria:**
- [ ] `make build` e `make lint` verdes
- [ ] `go test ./...` verde
- [ ] Ciclo `roadmap new` → `roadmap move ... wip` → `validate` passa **sem edição manual**
      (em diretório temporário, **nunca** escrevendo artefato neste repositório)
- [ ] Idem para `roadmap new --from-req`
- [ ] Blocos `**Acceptance criteria:**` por ML preservados
- [ ] Não tocar em `npm/`, `pypi/`, `internal/config/` nem `internal/validator/`

### ML-1B — Gerador Node
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `npm/src/generators/roadmap.js` + testes Node afetados

**Acceptance criteria:**
- [ ] `cd npm && npm test` verde
- [ ] Ciclo completo passa sem edição manual (em tmp)
- [ ] Idem `--from-req`
- [ ] Blocos por ML preservados
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — Gerador Python
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/generators/roadmap.py` + testes Python afetados

**Acceptance criteria:**
- [ ] Testes Python verdes
- [ ] Ciclo completo passa sem edição manual (em tmp)
- [ ] Idem `--from-req`
- [ ] Blocos por ML preservados
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

### Auditoria da Wave 1 (Zeus, 2026-08-01)

- Bloco presente **2×** em cada gerador (template simples + `--from-req`), comentário
  byte-idêntico nos três (`sort -u` → 1 linha), **sem espaço à direita** em nenhuma linha nova.
- `scripts/check-artifact-parity.sh` passa: 8 tipos de artefato × 3 runtimes.
- `make quality` exit 0; `trackfw validate` verde; o gate roda em sandbox e **não** poluiu o repo.
- Os três MLs rodaram de fato em paralelo sem colisão de arquivo.

**Lacuna identificada:** só o ML-1C fixou o contrato em teste
(`pypi/tests/test_generators_roadmap.py`). Go e Node não têm asserção sobre o bloco — nenhum teste
quebrou porque nenhum estava acoplado ao corpo gerado. `check-artifact-parity.sh` pega
*divergência entre* CLIs, mas **não** pega remoção coordenada nos três. É exatamente o que o
cenário permanente do ML-2A deve cobrir.

**Achado colateral reportado independentemente por dois agentes:** em `NewRoadmapFromREQ`
(Go, ~175) o campo `req:` do frontmatter recebe apenas `filepath.Base(reqPath)`, enquanto o
validador resolve esse valor como caminho relativo ao cwd — resultando em
`roadmap "..." links to REQ "..." which does not exist` até o usuário editar à mão. Reproduzido
também no Node e no Python. **Pré-existente e fora do escopo** — candidato a REQ própria.

---

## Wave 2 — Barreira de paridade e falsificação (1 ML)
> Dependências: **ML-1A, ML-1B e ML-1C completos e auditados**

### ML-2A — Paridade dos 3 CLIs e seam de falsificação
**Status:** ✅ concluído (auditado 2026-08-01)
**Agente:** Ártemis

**Ações:**
1. `scripts/check-artifact-parity.sh` — os três geradores produzem artefato idêntico.
2. `make quality` exit 0.
3. `trackfw validate` verde no repositório — roadmaps existentes não invalidados (AC6).
4. **Seam de falsificação (AC5):** reverter temporariamente **um** dos geradores para emitir só
   `**Acceptance criteria:**` e provar que o ciclo ponta a ponta (gerar → mover para `wip` →
   `validate`) **falha** com a violação `wip_acceptance`. Se não falhar, o passo 3 é vacuoso.
   Restaurar e reconfirmar.
5. Diferente do caso do DOMPurify, aqui o seam **é viável em CI** — é shell puro, sem DOM e sem
   dependência. Avaliar e, se couber, acrescentar cenário permanente a
   `scripts/check-gates-falsify.sh`.

**Acceptance criteria:**
- [x] `scripts/check-artifact-parity.sh` passa (8 tipos × 3 runtimes)
- [x] `make quality` exit 0
- [x] `trackfw validate` verde
- [x] Seam provado vivo
- [x] Gerador restaurado; sem resíduo de teste
- [x] **Cenário permanente em CI** — 6 asserções novas
      (`roadmap-acceptance-heading/{go,node,python}/{simple,from-req}`), contador 24 → 30

**Auditoria de Zeus (2026-08-01) — falsificação independente do próprio gate:**

Não bastava ver 30 OK. Removi **um** dos dois blocos do gerador Go e rodei o gate:

```
EXIT=1
expected 2 occurrences of the heading block, got 1
```

O gate falha por **guarda de pré-condição** — detecta que a fonte não casa mais o contrato e
aborta, em vez de rodar um ciclo já inválido. Restaurado, volta a 30/30 e `make quality` exit 0.
A lacuna apontada na auditoria da Wave 1 (remoção coordenada nos três CLIs passar despercebida)
**está fechada**: agora existe barreira de CI, não só paridade entre CLIs.

Diferente do ciclo do DOMPurify, onde o seam não pôde virar gate — aqui é shell puro, sem DOM e
sem dependência.

---

## Fechamento

Concluído e auditado em 2026-08-01. `make quality` exit 0; falsificação 30/30.

**Entrega:** os três geradores emitem o heading consolidado nos dois caminhos, e existe **gate de
CI** que prova a regressão — 6 asserções cobrindo 3 CLIs × 2 caminhos.

**Achado colateral root-caused, não corrigido** — `roadmap new --from-req` sempre dispara
`ref_targets_exist`, por **três** causas independentes: (1) `NewRoadmapFromREQ` grava apenas o
basename no campo `req:` do frontmatter, nos 3 CLIs; (2) `extractRefPath` retorna no primeiro
campo casado, e a linha `req:` do frontmatter precede a linha `REQ:` do corpo, então lê sempre o
valor errado; (3) `referenceExists(ref, roots)` nunca usa `roots` — faz `os.Stat(ref)` relativo ao
cwd. Documentado em `vault/notes/roadmap-from-req-ref-targets-exist-falso-positivo-2026-08-01.md`.
Dois agentes haviam reportado o sintoma sem diagnóstico; agora está com causa raiz e sugestão de
correção. **Candidato a REQ própria.**
