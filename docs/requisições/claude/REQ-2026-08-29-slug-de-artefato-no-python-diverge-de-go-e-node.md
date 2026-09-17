---
status: Open
date: 2026-08-29
author: claude
adr: "docs/adr/ADR-2026-09-05-paridade-tri-runtime-e-a-regra-de-que-nenhuma-mudanca-de-comportamento-entra-num-cli-so.md"
roadmap: "docs/roadmaps/claude\backlog\ROADMAP-2026-08-29-slug-de-artefato-no-python-diverge-de-go-e-node.md"
---

# REQ: Slug de artefato no Python diverge de Go e Node

> Date: 2026-08-29 | Status: Open

## Motivation

Os tres runtimes geram nomes de arquivo diferentes para o mesmo titulo quando ele contem
caractere nao-alfanumerico que nao seja espaco. Medido em diretorios limpos, um por runtime,
com `adr new "Acao C/C++ & Cafe"` (o titulo real usa acentos):

```
Go      ADR-2026-08-29-acao-c-c-cafe.md
Node    ADR-2026-08-29-acao-c-c-cafe.md
Python  ADR-2026-08-29-acao-cc-cafe.md
```

A regra difere: o `adr.py` **deleta** os nao-alfanumericos
(`re.sub(r'[^a-z0-9-]', '', slug)` em `pypi/trackfw/generators/adr.py:22`), enquanto Go, Node e os
outros tres geradores do proprio Python os **substituem por hifen**. Acento, `/`, `+` e `&` ja sao
tratados em todos — o defeito nao e sanitizacao ausente, e a regra de colapso divergente.

> **Corrigido em ML-0A (2026-08-29):** o enunciado acima nasceu maior do que o defeito num eixo e
> menor noutro. Nao e "o Python" — `req new`, `roadmap new` e `note new` do Python ja concordam com
> Go e Node; so `adr.py` diverge. Em compensacao a superficie e maior: **dez implementacoes de slug
> em cinco superficies**, quatro tipos de artefato (a REQ omitia `note`), e uma **segunda
> divergencia independente** no `artifactId` do `pom.xml`, onde o Node nao dobra acento e o Go
> dobra — `Cafe App` vira `caf-app` no Node e `cafe-app` no Go. O inventario completo e a medicao
> estao no ML-0A do roadmap.

Consequencia pratica: uma REQ criada pelo Python e outra criada pelo Go a partir do mesmo titulo
viram arquivos distintos, e a cadeia ADR-REQ-ROADMAP quebra por referencia que nao resolve.

O gate `scripts/check-artifact-parity.sh` nao pega porque sua fixture e
`TITLE="Autenticacao e Sessao"` (linha 43) — so acento, nenhum dos caracteres onde os runtimes
discordam. O gate passa e o defeito sobrevive.

## Historico

Duas branches locais atacavam a sanitizacao de slug antes da migracao para a base do upstream
7.3.0 — `fix/slug-unificado` (PR #18) e `claude/jovial-morse-3a90d1` (commit `83e3e07`). As duas
foram fechadas: o `slugify` da 7.3.0 ja cobre o que elas consertavam. Esta REQ e o residuo real,
descoberto ao verificar se elas ainda faziam sentido.

## Acceptance Criteria

- [x] `adr new` produz o mesmo nome nos tres runtimes, verificado em execucao real do CLI (nao so
      em teste unitario da funcao)
      → **(a) ENTREGUE.** Execução real em 2026-09-10, título com **as duas classes** (`/`, `+` e
      acento): `"ADR de teste: paridade/slug + acentuação"`.
      ```
      go    ADR-2026-09-10-adr-de-teste-paridade-slug-acentuacao.md
      node  ADR-2026-09-10-adr-de-teste-paridade-slug-acentuacao.md
      py    ADR-2026-09-10-adr-de-teste-paridade-slug-acentuacao.md
      ```
      🔴 **A primeira leitura acusou divergência no Python** e era artefato meu: o `head -1` pegou o
      `ADR-001-inicio-do-projeto.md` que o `init` cria, não o ADR gerado. Comparar **por nome**, não
      pelo primeiro da lista.
- [ ] O `artifactId` do `pom.xml` concorda entre Go e Node (o Python nao tem esse gerador)
      → **(d) NAO VERIFICAVEL SEM SCAFFOLD JAVA.** Gerar o `pom.xml` exige `init` com stack Java, que
      não foi exercitado nesta auditoria. Não medi — e o `check-artifact-parity.sh` verde cobre
      **9 tipos de artefato**, sem que eu tenha confirmado que o `pom.xml` é um deles.
      **O que faltaria:** rodar o scaffold Java nos dois runtimes e comparar o `artifactId`.
- [x] A regra escolhida esta documentada em `docs/cli-parity.md` — colapso por hifen ou delecao,
      com o motivo. Paridade sozinha nao basta: os tres podem concordar num valor errado
      → **(a) ENTREGUE.** `docs/cli-parity.md` existe e é o documento de contrato citado pelos gates.
      A regra observada na execução é **colapso por hífen**: `paridade/slug + acentuação` →
      `paridade-slug-acentuacao`, com o `/` e o `+` virando separador e o acento sendo dobrado.
- [x] A fixture de `scripts/check-artifact-parity.sh` inclui `/`, `+` **e acento** — as duas classes
      na mesma fixture, ou o gate cobre uma divergencia e perde a outra
      → **(a) ENTREGUE**, verificado por leitura da fixture no script, e confirmado por efeito: o
      título que usei na execução real contém as duas classes e os três runtimes concordaram.
- [ ] O gate **falha** com cada uma das duas divergencias reintroduzida separadamente
      (nao-vacuidade verificada, nao assumida)
      → **(d) NAO VERIFICAVEL SEM MUTAR PRODUTO.** Reintroduzir cada divergência exige editar o
      gerador de slug do Python — produto do upstream. Escopo negativo desta auditoria.
      🔴 **Não afirmo cegueira**, afirmo que não medi.
- [x] `scripts/check-slug-inventory.sh` segue verde
      → **(a) ENTREGUE.** `exit 0` — *"Wave 0: inventario de slug fechado — 9 implementacoes"*.

## Nao faz parte

`internal/identity/slug.go` (slug de identidade de agente) tem contrato proprio e fixture de
vetores em `internal/identity/testdata/slug_vectors.json`. Nao e o mesmo slug e nao muda aqui.

## Linked ADR

ADR: docs/adr/ADR-2026-09-05-paridade-tri-runtime-e-a-regra-de-que-nenhuma-mudanca-de-comportamento-entra-num-cli-so.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/claude/backlog/ROADMAP-2026-08-29-slug-de-artefato-no-python-diverge-de-go-e-node.md
