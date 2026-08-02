---
status: wip
date: 2026-08-02
req: "docs/req/REQ-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md"
squad: ""
---

# Roadmap: Suportar lista YAML inline nas chaves de config dos tres CLIs

> Created: 2026-08-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md
ADR: docs/adr/ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis.md

**Último item da fila.** KG pediu fechar antes da tag e antes do merge do PR #105, para aceitar
tudo de uma vez.

Os três CLIs ignoram `agents: [zeus, apolo]` em silêncio. Consistente entre CLIs, logo não é
paridade — é config válida descartada sem sinal.

### Estrutura — executor único, deliberadamente

Este projeto mostrou em **todos** os ciclos que três implementações paralelas divergem: fonte de
dado, texto de mensagem, raio de alcance da mudança. Aqui a exigência é **semântica idêntica** em
nove casos de parsing. Um executor com a tabela na mão é mais seguro que coordenar três.

## Critérios de Aceite

- [ ] Tabela de 9 casos idêntica nos 3 CLIs, em **cada** chave de lista
- [ ] Caso 8 (vírgula dentro de aspas) tratado — é o que quebra separação ingênua
- [ ] Cobertura por chave: `adr_dirs`, `agents`, `acceptance_markers`, sub-listas de `link_fields`
- [ ] Formas em bloco (indentada e não indentada) não regridem
- [ ] `status` em `by_agent` com inline: 3 saídas byte-idênticas, ordem **declarada**
- [ ] Cenário de falsificação por caso, com corrupção **determinística**
- [ ] `make build`, `make test`, `make lint`, `make parity`, `make quality` verdes

---

## Wave 1 — Parsing inline (1 ML, executor único nos 3 CLIs)
> Dependências: nenhuma

### ML-1A — Aceitar lista inline nos três parsers
**Status:** pending
**Agente:** Apolo (executor **único**)
**Arquivos afetados:** `internal/config/config.go`, `npm/src/config/index.js`,
`pypi/trackfw/config.py` + testes dos três

**Contrato (do ADR) — esta tabela é a definição, não sugestão:**

| # | Entrada | Resultado |
|---|---|---|
| 1 | `[a, b]` | `[a, b]` |
| 2 | `[a,b]` | `[a, b]` |
| 3 | `[ a , b ]` | `[a, b]` |
| 4 | `["a", "b"]` | `[a, b]` |
| 5 | `['a', 'b']` | `[a, b]` |
| 6 | `[a]` | `[a]` |
| 7 | `[]` | lista **vazia**, não default |
| 8 | `["a, b", "c"]` | **dois** itens: `a, b` e `c` |
| 9 | `["## Acceptance Criteria", "## Critérios de Aceite"]` | os dois |

**Acceptance criteria:**
- [ ] Tabela reproduzida nos 3 CLIs, saída literal lado a lado no relatório
- [ ] Caso 8 tratado — separação respeita aspas
- [ ] Testado **por chave**, não só `agents`
- [ ] Bloco indentado e não indentado não regridem
- [ ] `go test ./...`, `npm test`, `pytest pypi/tests` verdes
- [ ] Não tocar em validadores, `status`, nem `pypi/build/lib/`

---

## Wave 2 — Barreira (1 ML)
> Dependências: **Wave 1 completa**

### ML-2A — Paridade e seam por caso
**Status:** pending
**Agente:** Ártemis

**Ações:**
1. Gates de paridade passam; `make quality` exit 0; `validate` verde nos 3.
2. Confirmar que os **78** cenários existentes seguem passando. **Rodar, não presumir** — o
   parser de config mudou, e cenários que escrevem `trackfw.yaml` podem ser afetados.
3. Cenário novo cobrindo a tabela, com braço de detecção **determinístico**.
4. Contador e linha final atualizados.

**Acceptance criteria:**
- [ ] Gates passam; `make quality` exit 0
- [ ] 78 cenários herdados confirmados
- [ ] Cenário novo por caso relevante; corrupção determinística; não vacuoso
- [ ] Contador atualizado
- [ ] `git status --porcelain` sem resíduo
