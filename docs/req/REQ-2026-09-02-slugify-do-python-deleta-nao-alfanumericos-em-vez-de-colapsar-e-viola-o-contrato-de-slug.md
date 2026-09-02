---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-slug-de-artefato-colapsa-nos-tres-runtimes-e-o-gate-discrimina.md"
---

# REQ: uma das 4 cópias de `slugify` do Python derivou — `adr new` e `req new` dão nomes diferentes para o mesmo título

> Date: 2026-09-02 | Status: Open

## O achado, na forma mais curta

Mesmo título, mesma sessão, **mesmo runtime**:

```
$ python -m trackfw req new "C/C++ interop"
created docs/req/REQ-2026-09-02-c-c-interop.md
$ python -m trackfw adr new "C/C++ interop"
created docs/adr/ADR-2026-09-02-cc-interop.md
                       ^^^^^^^^^^^ dois slugs para o mesmo título
```

Não é divergência entre runtimes — é divergência **dentro do Python**.

## Mecanismo: 4 cópias, uma derivou

`slugify` está duplicado por gerador. Medidos os quatro com o mesmo título:

| | `adr` | `note` | `req` | `roadmap` |
|---|---|---|---|---|
| **Python** | `autenticacao-cc-v12` 🔴 | `…-c-c-v1-2` | `…-c-c-v1-2` | `…-c-c-v1-2` |
| **Node** | `…-c-c-v1-2` | `…-c-c-v1-2` | `…-c-c-v1-2` | `…-c-c-v1-2` |
| **Go** | — uma cópia só, compartilhada — |

Go tem **um** `toSlug` e por construção não pode derivar. Node tem quatro e as quatro
concordam — hoje. Python tem quatro e a do `adr` já derivou:

```python
# pypi/trackfw/generators/adr.py — DELETA          # os outros três — COLAPSAM
slug = ascii_str.lower().replace(' ', '-')          slug = ascii_str.lower()
slug = re.sub(r'[^a-z0-9-]', '', slug)              slug = re.sub(r"[^a-z0-9]+", "-", slug)
```

`docs/cli-parity.md`, seção "Slug — normalização NFKD portável nos três runtimes", especifica
**colapsar**: `substituição de sequências [^a-z0-9]+ por hífen`. Os três que colapsam estão certos;
o do `adr` está fora do contrato.

`trackfw/identity/__init__.py::slugify` **também deleta, e ali está correto** — o docstring declara
o passo 4 como "any character outside [a-z0-9-] is dropped", espelhando `internal/identity/slug.go`.
São duas regras diferentes de propósito. A confusão entre elas é a explicação mais provável do
defeito, e é por isso que ela está escrita aqui.

## Por que nenhum gate viu — e este é o ponto

`scripts/check-artifact-parity.sh` usava `TITLE="Autenticação e Sessão"`. Medidos os **três**
exemplos da tabela do contrato, todos dão 3/3 iguais mesmo com o defeito presente:

```
"Autenticação e Sessão"  → autenticacao-e-sessao   (3/3)
"ADR Config (v2)"        → adr-config-v2           (3/3)
"Minha Requisição #1"    → minha-requisicao-1      (3/3)
```

Neles todo trecho não-alfanumérico é adjacente a espaço ou a borda — e ali deletar e colapsar
**coincidem**. A anotação do contrato já declarava a cobertura como `partial`; a medição mostra algo
pior que parcial: um gate que exercitasse os três exemplos continuaria verde com o defeito presente.
Não faltava cobertura, faltava um caso **discriminante**.

Discrimina quando o não-alfanumérico está *entre* alfanuméricos, sem espaço: `C/C++`, `v1.2`.

## Um segundo sítio, no Node

`npm/src/generators/init.js::generatePomXml` usa uma variante inline, sem NFKD:

```
"Café App"     inline: caf-app      toSlug: cafe-app
"Ação Rápida"  inline: a-o-r-pida   toSlug: acao-rapida
```

Mesmo nome de projeto, dois identificadores conforme o gerador.

## Acceptance Criteria

- [ ] **AC1** — `pypi/trackfw/generators/adr.py::slugify` colapsa `[^a-z0-9]+`, alinhando-se aos
      outros três geradores do próprio Python e ao contrato.
- [ ] **AC2** — `generatePomXml` usa o `toSlug` compartilhado em vez da variante inline.
- [ ] **AC3** — 🔴 **O gate passa a exercitar um caso que DISCRIMINA.** Acrescentar mais um exemplo
      da tabela atual não serve — os três dão o mesmo resultado sob as duas semânticas. Feito:
      `TITLE="Autenticação C/C++ v1.2"`, que mantém a cobertura de NFKD e acrescenta os dois padrões
      discriminantes.
- [ ] **AC4** — 🔴 **Falsificação nas duas direções.**
      (a) com a correção, `check-artifact-parity` passa nos 8 tipos × 3 runtimes;
      (b) **controle** — revertendo só o Python, o gate reprova e **nomeia o runtime**:
      `artifact parity drift: adr (python) — arquivo ausente: ADR-2026-09-02-autenticacao-c-c-v1-2.md`.
- [ ] **AC5** — Os 3 exemplos já documentados continuam produzindo o mesmo slug de antes. A correção
      não muda nome de artefato existente.

## Negative Scope

**Não renomeia artefato já criado.** Um ADR chamado `cc-interop` no disco de alguém fica como está;
a correção vale para os próximos. Migração é decisão de vocês e mexeria em referência de REQ.

**Não toca em `identity/slugify`**, que deleta por especificação declarada.

**Não desduplica os 4 `slugify`.** A duplicação é o mecanismo que permitiu a deriva, e unificar seria
a correção estrutural — mas é mudança de superfície maior, e vale a decisão de vocês em separado.
Esta REQ corrige a cópia que derivou e instala o gate que teria pego.

## Linked ADR
<!-- Correção de conformidade a contrato já documentado; sem decisão nova a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-slug-de-artefato-colapsa-nos-tres-runtimes-e-o-gate-discrimina.md
