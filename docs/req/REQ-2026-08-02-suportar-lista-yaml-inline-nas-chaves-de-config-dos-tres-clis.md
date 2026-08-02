---
status: Open
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md"
---

# REQ: Suportar lista YAML inline nas chaves de config dos tres CLIs

> Date: 2026-08-02 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Último item da fila. Os **três** CLIs ignoram `agents: [zeus, apolo]` em silêncio — YAML válido,
nenhum erro, nenhum aviso, e o CLI cai no default ou no fallback.

É **consistente** entre CLIs, portanto não é paridade. É defeito de produto: config válida
descartada sem sinal, a pior classe de erro de configuração.

Afeta `adr_dirs`, `agents`, `acceptance_markers` e as sub-listas de `link_fields`.
`rules` é mapeamento, não sequência — fora.

## Acceptance Criteria

- [ ] **AC1** — Os 3 CLIs produzem resultado **idêntico** para a tabela abaixo, em **cada** chave
      de lista. Esta tabela é o contrato:

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
  | 9 | `["## Acceptance Criteria", "## Critérios de Aceite"]` | os dois marcadores |

- [ ] **AC2** — **Caso 8 é obrigatório.** Separar por vírgula sem respeitar aspas quebra valores
      que contêm vírgula. O caso 9 é o caso real do projeto.
- [ ] **AC3** — Cobertura **por chave**: `adr_dirs`, `agents`, `acceptance_markers` e as
      sub-listas de `link_fields`. Fixture só com `agents` **não** basta.
- [ ] **AC4** — **Não regride:** forma em bloco **indentada** e **não indentada** continuam
      funcionando em todas as chaves.
- [ ] **AC5** — `status` em `by_agent` com `agents:` inline: as **3** saídas byte-idênticas, e
      respeitando a ordem **declarada** (não o fallback).
- [ ] **AC6** — `validate` verde nos 3 no repositório; `status` no repositório real segue
      byte-idêntico nos 3.
- [ ] **AC7** — `scripts/check-artifact-parity.sh` e `scripts/check-validate-parity.sh` passam.
- [ ] **AC8** — Cenário de falsificação por caso da tabela, nos 3 CLIs, com braço de detecção.
      **A corrupção precisa ser determinística** — não dependente de filesystem ou ambiente
      (ver `vault/notes/cenarios-de-falsificacao-quebram-em-refactor-do-alvo-2026-08-02.md`).
- [ ] **AC9** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não adotar biblioteca YAML.** No Go seria barato (`yaml.v3` já está como indirect), mas no
  Node e no Python significa dependência de runtime nova e reescrita do parsing de config. É
  mudança de política de dependências — ADR próprio, se um dia.
- **Não suportar outras formas de YAML:** listas aninhadas inline (`[[a],[b]]`), mapas inline
  (`{a: 1}`), âncoras, multi-linha. Continuam sem suporte **e sem aviso** — a classe é reduzida,
  não eliminada, e isso está registrado no ADR.
- **Não definir precedência** entre inline e bloco para a mesma chave no mesmo arquivo. Sem caso
  real; definir sem necessidade é especulação.
- Não incluir `rules` — é mapeamento, não sequência.
- Não alterar validadores, o comando `status` nem mensagens de saída.
- Não alterar o status de nenhum ADR ou REQ do repositório.
- Não mexer em `pypi/build/lib/`.

## Notas de implementação

| CLI | Parser |
|---|---|
| Go | `internal/config/config.go` — laço com `hasIndent` / `continuesOpenList` |
| Node | `npm/src/config/index.js` |
| Python | `pypi/trackfw/config.py` |

**Executor único nos três CLIs, deliberadamente.** Este projeto mostrou em **todos** os ciclos
que três implementações paralelas divergem — em fonte de dado, em texto de mensagem, em raio de
alcance. Coordenar três agentes para produzir semântica idêntica provou ser mais frágil do que um
executor com a tabela na mão.

`check-gates-falsify.sh` está em 78 cenários e leva mais de 2 min; rodar em background.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-suporte-a-lista-yaml-inline-nos-parsers-de-config-dos-tres-clis.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-02-suportar-lista-yaml-inline-nas-chaves-de-config-dos-tres-clis.md
