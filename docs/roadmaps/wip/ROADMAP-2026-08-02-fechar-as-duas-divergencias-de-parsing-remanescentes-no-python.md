---
status: wip
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-fechar-as-duas-divergencias-de-parsing-remanescentes-no-python.md"
squad: ""
---

# Roadmap: Fechar as duas divergencias de parsing remanescentes no Python

> Created: 2026-08-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-02-fechar-as-duas-divergencias-de-parsing-remanescentes-no-python.md
ADR: docs/adr/ADR-2026-08-02-python-alinha-delimitador-nao-pareado-e-ordenacao-do-fallback-de-agentes.md

Os dois últimos itens da fila. KG pediu que fossem fechados **antes da tag**, para não versionar
defeito conhecido. Ambos já estavam medidos; nenhum tem caso real no repositório.

**Item 1 — delimitador não pareado.** `ADR: "docs/adr/X.md'` resolve em Go e Node; o Python
devolve vazio (`normalize_yaml_flat_value` só remove par casado).

**Item 2 — mudou de natureza na investigação.** Eu havia reportado como "parser YAML do Python
não trata lista inline". **Nenhum dos três trata.** Todos caem no fallback de varrer
subdiretórios; a divergência está **no fallback**: `_list_dirs` não ordena, enquanto a irmã
`_list_files` no mesmo arquivo ordena, e Go/Node ordenam.

```
fixture: docs/roadmaps/{zeus,apolo}/wip/ (criados nessa ordem)
Go/Node → [apolo] … [zeus]   (ordenado)
Python  → [zeus]  … [apolo]  (ordem do filesystem)
```

**Achado novo, fora do escopo:** os três **silenciosamente ignoram** `agents: [a, b]`. Defeito
real, mas **consistente** — logo não é paridade, e resolvê-lo exige decisão de produto (suportar
inline versus avisar). Vai para a fila.

### Escopo

As duas correções são **só no Python**. Go e Node concordam entre si nos dois itens e são a
referência. Um ML de implementação + barreira.

## Critérios de Aceite

- [ ] Delimitador não pareado: os 3 produzem a mesma saída; tabela de 8 entradas idêntica nos 8
- [ ] `_list_dirs` ordena; fallback de agentes byte-idêntico nos 3
- [ ] Não regride: flat e `by_agent` com `agents:` em bloco seguem idênticos; repo real em 749 B
- [ ] `validate` verde nos 3
- [ ] Cenários de falsificação com fixtures **discriminantes** para os dois itens
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Correções no Python (1 ML)
> Dependências: nenhuma

### ML-1A — Delimitador não pareado e ordenação do fallback
**Status:** ✅ concluído (auditado 2026-08-02)
**Agente:** Apolo
**Arquivos afetados:** `pypi/trackfw/validator.py`, `pypi/trackfw/commands/status.py` + testes

**Ações:**
1. No caminho de `_extract_ref_path`, remover delimitador **sem exigir par casado**, alinhando a
   Go/Node. **Manter contido ao caminho da extração** — o PR #104 estreitou isso de propósito
   para não afetar `parse_frontmatter`, e há teste de regressão. **Não reverter o estreitamento.**
2. `_list_dirs` (~linha 37) passa a ordenar, como `_list_files` já faz.
3. Testes.

**Acceptance criteria:**
- [ ] Tabela de 8 entradas reexecutada: idêntica a Go/Node nos **8** casos
- [ ] Teste: `parse_frontmatter` **continua** preservando backtick (regressão do #104)
- [ ] Teste: fixture com subdiretórios fora de ordem alfabética → `_list_dirs` ordena
- [ ] `pytest pypi/tests` verde
- [ ] Não tocar em `internal/`, `npm/`, `pypi/build/lib/`

---

### ML-1B — Go e Node aceitam sequência em bloco não indentada
**Status:** pending
**Agente:** Apolo
**Arquivos afetados:** `internal/config/config.go`, `npm/src/config/index.js` + testes

**Terceiro defeito, descoberto ao validar o ML-1A — e aqui quem erra são Go e Node.**

```yaml
agents:
- zeus
- apolo
```

É **YAML válido** (confirmado com parser real). Go (`config.go:129-132`) e Node classificam
qualquer linha sem indentação como top-level, encerrando a lista aberta → a lista é
**silenciosamente descartada** e o CLI cai no fallback. O Python lê certo.

**Alcance:** o mesmo `hasIndent` governa `adr_dirs`, `agents`, `acceptance_markers`,
`link_fields` e `rules`. Não é uma chave — é o laço do parser.

**Inverte a heurística do ciclo.** Vínhamos usando "dois concordam, o terceiro se alinha". Aqui a
maioria está errada: **maioria não é autoridade quando há especificação**, e o YAML dá razão ao
Python.

**Acceptance criteria:**
- [ ] Linha iniciada por `- ` deixa de encerrar lista aberta apenas por falta de indentação
- [ ] Vale para **todas** as cinco chaves de lista, com fixture **por chave** — não só `agents`
- [ ] Forma **indentada** continua funcionando (não regride)
- [ ] `by_agent` com lista não indentada: as 3 saídas byte-idênticas
- [ ] `make build`, `make lint`, `go test ./...`, `npm test` verdes
- [ ] Não tocar em `pypi/` — o Python já está correto

---

## Wave 2 — Barreira (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Paridade e seam dos dois itens
**Status:** pending
**Agente:** Ártemis

**Ações:**
1. Gates de paridade passam; `make quality` exit 0; `validate` verde nos 3.
2. Confirmar que os **69** cenários existentes seguem passando.
3. **Cenários novos, com fixtures discriminantes:**
   - delimitador não pareado — sem essa fixture o cenário não exercita o item 1
   - `by_agent` **sem `agents:` configurado**, com subdiretórios fora de ordem alfabética — os
     cenários 31 existentes usam lista em bloco e portanto **não** passam pelo fallback
   - **lista em bloco NÃO indentada** — item 3. Os cenários existentes usam forma indentada e
     portanto **não** exercitam o defeito.
4. Braços de detecção para os três.
5. Contador e linha final atualizados.

**Acceptance criteria:**
- [ ] Gates passam; `make quality` exit 0
- [ ] 69 cenários herdados confirmados
- [ ] Cenários novos para os **três** itens, com fixtures discriminantes; não vacuosos
- [ ] Contador atualizado
- [ ] `git status --porcelain` sem resíduo
