---
status: Done
date: 2026-07-27
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md"
---

# REQ: convergencia dos templates de artefato do CLI Python

> Date: 2026-07-27 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

O CLI Python gera `roadmap`, `req` e `adr` com frontmatter, header e seções **diferentes** de Go e
Node. Isso começou como divergência de formato, mas o levantamento mostrou que o efeito real é pior:
**duas regras do validator ficam vacuamente verdes** para artefatos criados pelo Python.

### Os dois defeitos de runtime — o núcleo desta REQ

**1. REQ gerada pelo Python nunca é reconhecida como `Open`.**
O validator detecta status por busca de substring — `strings.Contains(content, "Status: Open")`
(`internal/validator/validator.go:1006`, `pypi/trackfw/validator.py:767`, equivalente em Node). O
template Go/Node produz a linha `> Date: <data> | Status: Open`; o template Python produz uma
**tabela** (`| Status | Open |`), onde a string `Status: Open` não existe.
Consequência: a REQ escapa da regra `req_blocked_by_draft_adr` e do comando `sync`.

**2. ADR Draft gerado pelo Python nunca é detectado como Draft.**
`adrIsDraft` faz `Contains(content, "Status: Draft")`, case-sensitive
(`internal/validator/validator.go:1106`, `npm/src/validator/index.js:230`,
`pypi/trackfw/validator.py:355`). O ADR Python tem `status: Draft` no frontmatter e uma seção
`## Status`, mas nunca a string procurada.
Consequência: `blocked_by_draft_adr` passa **por ausência de match, não por conformidade**.

Este é exatamente o padrão **P2 — degradação silenciosa** catalogado no
`ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`: a regra não falha, ela cala. É o
mesmo formato de defeito que a REQ-2026-07-26 passou três waves eliminando do validator, sobrevivendo
aqui porque nenhum gate jamais executou um gerador.

### Causa raiz de ter passado despercebido

`scripts/check-cli-parity.sh` compara **nomes de subcomando** extraídos do `--help`. Nunca executa
`adr new`, `req new` ou `roadmap new`, nunca lê um arquivo gerado. É paridade de *superfície*, não de
*saída*. `check-validate-parity.sh` compara JSON dos 3 runtimes, mas sobre fixture escrita à mão —
nunca sobre artefato produzido pelos próprios CLIs.

### Divergências de frontmatter

| Artefato | Go / Node | Python |
|---|---|---|
| roadmap | `status: backlog` · `date` · `req` · `squad` | `name` · `title` · `status: Backlog` · `created` · `author` |
| req | `status: Open` · `date` · `author` · `adr` · `roadmap` | `name` · `title` · `status: Open` · `linked_adr: —` · `created` · `author` |
| adr | `status: Proposed` · `date` · `author` | `name` · `title` · `status` · `created` |
| nota de vault | idênticos nos 3 — único artefato com contrato documentado (`docs/cli-parity.md:38-40`) | idem |

Chaves sem nenhum leitor no código: `date`, `created`, `author`, `name`, `title`, `linked_adr` —
divergência de contrato. Chaves com leitor real, ausentes no Python: `req:` e `adr:` (arestas do grafo
em `serve/api_chain`) e `squad:` (`wip_limit` com `by_squad`, que cai todo em `(no squad)`).

Nomenclatura de arquivo ADR também diverge: Go/Node usam `ADR-<YYYY-MM-DD>-<slug>.md`, Python usa
numeração sequencial `ADR-NNN-<slug>.md`.

### Formato canônico: Go/Node, e já está declarado

`docs/schema/roadmap.schema.json`, `req.schema.json` e `adr.schema.json` descrevem o formato Go/Node
(`required: ["status","date"]`, enum de status minúsculo). **Nenhum ADR declara a variante Python como
intencional** e `docs/cli-parity.md` marca `adr`/`req`/`roadmap` como paritários, sem ressalva.
É bug, não contrato.

**Decisão de idioma:** os templates convergem para o formato Go/Node, **em inglês**
(`## Motivation`, `## Acceptance Criteria`, `## Context`). O canônico existe e está em inglês; três
runtimes precisam produzir bytes idênticos. A regra de PT-BR do projeto governa comunicação e
documentação de arquitetura, não o formato de artefato que os CLIs emitem.

## Acceptance Criteria

- [x] Teste negativo que **expõe** as duas regras cegas: artefato no formato Python atual deve ser
      detectado como `Open` / `Draft` — escrito **antes** da convergência, e visto falhar
- [x] `req new` do Python gera frontmatter e header idênticos a Go/Node, incluindo `> Date: … | Status: Open`
- [x] `adr new` do Python gera frontmatter e header idênticos a Go/Node, incluindo `> Date: … | Status: <status>`
- [x] `adr new` do Python usa `ADR-<YYYY-MM-DD>-<slug>.md`, como Go/Node
- [x] `roadmap new` do Python gera `status: backlog` · `date` · `req` · `squad`, em minúsculo
- [x] Os 3 CLIs produzem os 3 artefatos **byte a byte idênticos** para a mesma entrada
- [x] Gate que executa os geradores nos 3 CLIs e compara a saída, com prova negativa (P4)
- [x] `make quality` verde, sem variável de ambiente auxiliar

## Escopo negativo — registrado, não corrigido

1. **Migração dos 50 roadmaps existentes** (12 formatos distintos em `docs/roadmaps/`). Nenhuma chave
   divergente tem leitor que quebre retroativamente — `date`/`created`/`author`/`name`/`title` não têm
   consumidor. Convergir o gerador para frente não exige migrar o passado.
2. **Slash-command `/trackfw:roadmap` gera roadmap SEM frontmatter**, e é idêntico nos 3 `init`
   (`internal/generators/scaffold.go:278`, `npm/src/generators/init.js:790`,
   `pypi/trackfw/generators/init_gen.py:507`). É um **terceiro** formato concorrente e explica boa
   parte da mistura no repo. Mexer nele muda o que todo projeto instalado recebe — REQ própria, e
   provavelmente a próxima.
3. **`--from-req`, `--req`, `--title` e wizard TTY ausentes no Python** — paridade de funcionalidade,
   não de formato.
4. **`docs/schema/*.json` não é consumido por código nenhum**, e `site/guide/ai-agents.md:68` afirma
   falsamente que `trackfw validate` valida contra eles. Schema morto + doc incorreta.
5. ~~**Divergências menores Go↔Node** no template de roadmap.~~
   **PROMOVIDO AO ML-2B — 2026-07-27.** A medição empírica após o ML-2A invalidou a premissa deste
   item. Não são duas divergências cosméticas: são **quatro**, duas nunca catalogadas, e uma delas é
   o Node **descartando o título digitado pelo usuário** (`roadmap new "auth strategy"` gera
   `# Roadmap: New Roadmap`) — perda silenciosa de input, P2, no caminho principal do comando.
   Além disso, sem corrigi-las o gate da Wave 3 seria impossível: teria que nascer com lista de
   exceções documentadas, que é exatamente o "número mágico" condenado por P1. Um gate com exceção
   não impede regressão, legitima-a.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md` — esta REQ é um caso de
P2 (degradação silenciosa) e a correção só é aceita com a prova negativa exigida por P4.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-27-convergencia-dos-templates-de-artefato-do-cli-python.md`
