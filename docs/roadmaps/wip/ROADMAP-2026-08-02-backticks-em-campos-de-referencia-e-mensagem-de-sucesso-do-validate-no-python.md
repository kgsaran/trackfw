---
status: wip
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md"
squad: ""
---

# Roadmap: Backticks em campos de referencia e mensagem de sucesso do validate no Python

> Created: 2026-08-02 | Status: wip

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

- [ ] Backtick removido nos 3 CLIs, mantendo o mecanismo próprio de cada um
- [ ] As 3 REQs invisíveis passam a ter o ADR resolvido — provado por teste
- [ ] Fixture com backtick + ADR `Proposed` **viola**; antes não violaria
- [ ] `validate` verde nos 3 no repositório (reachability sobe, violações não)
- [ ] Tabela compartilhada de entradas produz saída idêntica nos 3 — divergência reportada, não resolvida
- [ ] Python usa `validate.ok`; os 3 imprimem `✓ No violations found.`
- [ ] Cenário de falsificação **com fixture contendo backtick**
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Extrator e mensagem (3 MLs EM PARALELO)
> Dependências: nenhuma. Arquivos disjuntos por CLI.

### ML-1A — CLI Go
**Status:** pending
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
**Status:** pending
**Agente:** Apolo
**Arquivos afetados:** `npm/src/validator/index.js` + testes Node

**Ações:** acrescentar backtick ao regex (~linha 863). Mesmos testes.

**Acceptance criteria:** equivalentes ao ML-1A (`npm test` verde).
- [ ] Não tocar em `internal/`, `pypi/`

### ML-1C — CLI Python (extrator **e** mensagem)
**Status:** pending
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

## Wave 2 — Barreira: paridade e falsificação (1 ML)
> Dependências: **ML-1A, ML-1B e ML-1C completos e auditados**

### ML-2A — Paridade e seam com fixture de backtick
**Status:** pending
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
- [ ] Gates de paridade passam; `make quality` exit 0
- [ ] Os 3 CLIs imprimem a **mesma** mensagem de sucesso
- [ ] 57 cenários herdados confirmados
- [ ] Cenário novo com backtick, provado não vacuoso; contador atualizado
- [ ] `git status --porcelain` sem resíduo
