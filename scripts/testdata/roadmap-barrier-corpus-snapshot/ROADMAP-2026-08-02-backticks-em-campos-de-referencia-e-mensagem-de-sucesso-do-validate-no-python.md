---
status: done
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md"
squad: ""
---

# Roadmap: Backticks em campos de referencia e mensagem de sucesso do validate no Python

> Created: 2026-08-02 | Status: done

## Context

REQ: docs/req/REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md
ADR: docs/adr/ADR-2026-08-02-extracao-de-referencia-tolerante-a-markdown-e-saida-do-validate-via-i18n.md

Pontos **2** e **3** da fila de achados colaterais do PR #103, na ordem pedida por KG.

**A — backtick torna a referência invisível.** `extractRefPath` remove aspas mas não backticks;
`` ADR: `docs/adr/X.md` (prosa) `` não termina em `.md` → nenhuma referência encontrada, em
silêncio. 13 REQs usam essa forma; **3** sem `adr:` no frontmatter ficam inalcançáveis.

**B — Python ignora a própria chave de i18n.** Os 3 CLIs têm `validate.ok` =
`"✓ No violations found."`; `pypi/trackfw/commands/validate.py:104` imprime `"✓ Governance OK"`
hardcoded.

### Causa única confirmada

`adr: ""` no frontmatter **não** causa early-return — reduz a vazio, falha `.md`, e o laço
continua. O backtick é a **única** causa. Não "corrigir" o early-return.

### Divergência a medir, não corrigir

Os 3 tokenizam igual (primeiro token), mas removem delimitadores diferente: Go `strings.Trim`
(conjunto), Node regex de uma ocorrência por ponta, Python só **par casado**. O ADR decide
**medir** via AC5, não unificar.

### Dependências e paralelismo

Wave 1 com **3 MLs em paralelo** (arquivos disjuntos por CLI). `make parity` só fecha com os três
prontos — **nenhum ML da Wave 1 tem `parity` nos critérios**; a paridade é a Wave 2.

## Critérios de Aceite

- [x] Backtick removido nos 3 CLIs, mantendo o mecanismo próprio de cada um
- [x] As 3 REQs invisíveis passam a ter o ADR resolvido — provado por teste
- [x] Fixture com backtick + ADR `Proposed` **viola**; antes não violaria
- [x] `validate` verde nos 3 no repositório (reachability sobe, violações não)
- [x] Tabela executada nos 3 — caso 6 diverge, **reportado e não resolvido** (vira item 4 da fila)
- [x] Python usa `validate.ok`; os 3 imprimem `✓ No violations found.`
- [x] Cenário de falsificação **com fixture contendo backtick**
- [x] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Extrator e mensagem (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos por CLI.

### ML-1A — CLI Go
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo
**Arquivos afetados:** `internal/validator/validator.go` + testes Go

**Ações:** acrescentar backtick ao conjunto de `strings.Trim` (~linha 1526). Testes de AC2, AC3 e
da tabela do AC5.

**Acceptance criteria:**
- [ ] `make build`, `make lint`, `go test ./...` verdes
- [ ] Teste: `` ADR: `docs/adr/X.md` (prosa) `` resolve para `docs/adr/X.md`
- [ ] Teste discriminante: fixture com backtick + ADR `Proposed` viola `adr_accepted_when_req_done`
- [ ] Teste: as 3 REQs reais do repositório têm o ADR resolvido
- [ ] Tabela do AC5 executada e resultado reportado
- [ ] `bin/trackfw validate` verde no repositório
- [ ] Não tocar em `npm/`, `pypi/`; não unificar mecanismos; não mexer no early-return

### ML-1B — CLI Node
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo
**Arquivos afetados:** `npm/src/validator/index.js` + testes Node

**Ações:** acrescentar backtick ao regex (~linha 863). Mesmos testes.

**Acceptance criteria:** equivalentes ao ML-1A (`npm test` verde).
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — CLI Python (extrator **e** mensagem)
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/validator.py`, `pypi/trackfw/commands/validate.py` + testes

**Ações:** acrescentar backtick a `normalize_yaml_flat_value` (~319), **mantendo** a semântica de
par casado; trocar o literal `"✓ Governance OK"` (`commands/validate.py:104`) pela chave
`validate.ok` do i18n.

**Acceptance criteria:** equivalentes ao ML-1A, **mais**:
- [ ] `PYTHONPATH=pypi python3 -m trackfw validate` imprime `✓ No violations found.`
- [ ] Nenhum literal `Governance OK` remanescente em `pypi/`
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

### Resultado da medição do AC5 (os 3 CLIs, tabela compartilhada)

| # | Entrada | Go | Node | Python |
|---|---|---|---|---|
| 1 | `` ADR: `X.md` `` | `X.md` | `X.md` | `X.md` |
| 2 | `ADR: "X.md"` | `X.md` | `X.md` | `X.md` |
| 3 | `ADR: 'X.md'` | `X.md` | `X.md` | `X.md` |
| 4 | `ADR: X.md` | `X.md` | `X.md` | `X.md` |
| 5 | `` ADR: `X.md` (prosa) `` | `X.md` | `X.md` | `X.md` |
| 6 | `ADR: "X.md'` **não pareado** | `X.md` | `X.md` | **`''`** |
| 7 | `ADR:` | vazio | `null` | vazio |
| 8 | `ADR: —` | vazio | `null` | vazio |

**Caso 6 diverge, como previsto.** Go e Node removem delimitador não pareado; Python exige par
casado. **Medido e deliberadamente não resolvido** — o ADR decidiu medir, não unificar, e o escopo
negativo proíbe. Nenhum caso real no repositório usa delimitador não pareado. **Vira item 4 da
fila de follow-ups.**

---

### ML-1D — Estreitar o raio da mudança no Python (corretivo)
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo

**Divergência NOVA, introduzida pela própria Wave 1 e pega na auditoria.** Go e Node alteraram
**só** `extractRefPath`; o parser de frontmatter de ambos continua sem backtick. O Python alterou
`normalize_yaml_flat_value` — helper compartilhado por **10 call sites**, incluindo
`parse_frontmatter`, `status`, `squad`, `governance_mode` e `traceid.py`. Resultado: no Python o
backtick passou a ser removido **em todo o frontmatter**.

Provado empiricamente antes de corrigir:

```
Python parse_frontmatter('adr: `docs/adr/X.md`')  → 'docs/adr/X.md'   ← removia
Go     extractFrontmatterField                     → mantinha os backticks
```

**Corrigido:** `normalize_yaml_flat_value` voltou a conhecer só aspas; `_extract_ref_path` ganhou
remoção própria de par de backticks. Reverificado: frontmatter **preserva** nos três, extrator
**resolve** nos três. Teste de regressão adicionado para impedir que alguém "simplifique"
reusando o helper compartilhado.

**Lição:** "mesma correção nos 3 CLIs" não basta — é preciso conferir se o **raio de alcance** é o
mesmo. Um CLI editar um helper compartilhado enquanto os outros editam o ponto de uso produz
divergência silenciosa que nenhum teste de unidade pega.

---

## Wave 2 — Barreira: paridade e falsificação (1 ML)
> Dependências: **ML-1A, ML-1B e ML-1C completos e auditados**

### ML-2A — Paridade e seam com fixture de backtick
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Ártemis

**Ações:**
1. `check-validate-parity.sh` e `check-artifact-parity.sh` passam; `make quality` exit 0;
   `validate` verde e com **mensagem idêntica** nos 3.
2. Confirmar que os **57 cenários** existentes seguem passando.
3. **Cenário permanente com fixture contendo backtick**, 3 CLIs: revertendo a remoção do backtick,
   a violação **desaparece**. Sem fixture com backtick o cenário é vacuoso.
4. Reconciliar a tabela do AC5 se os MLs reportarem divergência.
5. Contador e linha final atualizados.

**Acceptance criteria:**
- [x] Gates de paridade passam; `make quality` exit 0
- [x] Os 3 CLIs imprimem a mesma mensagem — verificado **byte a byte** com `od -c` + `diff`
- [x] 57 cenários herdados confirmados
- [x] **Dois** cenários novos; contador **57 → 65**, contado por `grep -c`, não estimado
- [x] Provados não vacuosos
- [x] `git status --porcelain` mostra só o script

**Entrega acima do pedido — cenário 29, mensagem de sucesso.** Nada em CI garantia que os 3 CLIs
imprimissem a mesma mensagem, e foi por isso que o Python passou meses com o literal hardcoded.

Decisão dela que vale registrar: o cenário compara os três **contra um literal pinado**, não os
três entre si. Um diff a três passaria se os três derivassem **juntos**, ou se todos imprimissem
vazio. Pinando o literal, a comparação continua válida mesmo nesse caso.

Ressalva honesta que ela levantou: a comparação usa `$(...)`, que normaliza newline final — drift
de linha em branco ao final não seria pego por esse gate. Verificado à parte com `od -c` que os
três têm exatamente um `\n`.

---

## Fechamento

Concluído e auditado em 2026-08-02. `make quality` exit 0; falsificação **57 → 65** cenários.

**Pontos 2 e 3 da fila fechados.** Três REQs saíram da invisibilidade; os três CLIs passaram a
imprimir a mesma mensagem de sucesso, e agora há gate de CI garantindo ambas as coisas.

**Item 4 da fila, criado por este ciclo:** delimitador **não pareado** (`ADR: "X.md'`) resolve em
Go e Node e **não** resolve em Python. Medido, documentado, sem caso real no repositório.
Deliberadamente não resolvido — o ADR decidiu medir, não unificar.

**A lição do ML-1D é a mais transferível:** "mesma correção nos 3 CLIs" não basta. É preciso
conferir se o **raio de alcance** é o mesmo. Aqui, um CLI editou um helper compartilhado por 10
call sites enquanto os outros dois editaram só o ponto de uso — divergência silenciosa que
nenhum teste de unidade e nenhum gate existente pegaria.
