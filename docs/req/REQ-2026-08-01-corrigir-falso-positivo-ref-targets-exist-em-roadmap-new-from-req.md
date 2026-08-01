---
status: Done
date: 2026-08-01
author: "Zeus"
adr: "docs/adr/ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req.md"
---

# REQ: Corrigir falso-positivo ref_targets_exist em roadmap new --from-req

> Date: 2026-08-01 | Status: Done
| Linear Issue: 
| Jira Issue: 

## Motivation

`roadmap new --from-req` gera um roadmap que o `validate` reprova, mesmo com a REQ existindo.
Reproduzido em diretório limpo em 2026-08-01: o frontmatter recebe `req: "REQ-....md"` (basename)
enquanto o corpo recebe o caminho completo; o validador lê o frontmatter e o `os.Stat` falha.

É o **terceiro caso seguido** de "a ferramenta reprova o artefato que ela mesma gerou", e atinge
justamente o caminho recomendado quando já existe uma REQ com critérios prontos.

Três causas somadas — gerador gravando basename, `extractRefPath` retornando no primeiro match
(e o frontmatter precede o corpo), e `referenceExists` ignorando o parâmetro `roots`.
Diagnóstico completo em
`vault/notes/roadmap-from-req-ref-targets-exist-falso-positivo-2026-08-01.md`.

## Acceptance Criteria

- [x] **AC1** — `roadmap new --from-req <req>` grava o **caminho relativo completo** no campo
      `req:` do frontmatter, nos 3 CLIs. Verificado por inspeção do artefato gerado.
- [x] **AC2** — Ciclo ponta a ponta em diretório temporário nos 3 CLIs: `req new` →
      `roadmap new --from-req` → `roadmap move ... wip` → `validate` **sem** a violação
      `links to REQ ... which does not exist`.
- [x] **AC2b** — **Bug irmão, descoberto em 2026-08-01 durante o setup deste ciclo:**
      `roadmap new --title <t> --req <path>` (caminho simples, sem `--from-req`) grava
      `req: ""` **vazio** no frontmatter, embora preencha o corpo corretamente. Mesmo campo,
      mesma família de defeito. O caminho simples passa a gravar o `reqPath` completo no
      frontmatter quando `--req` é informado, nos 3 CLIs. Quando `--req` **não** é informado,
      `req: ""` permanece — é o comportamento correto.
- [x] **AC3** — O parâmetro `roots` é **removido** da assinatura de
      `referenceExists` (Go), `referenceExists` (Node) e `_reference_exists` (Python), com os
      **três chamadores de cada** ajustados. Nenhum parâmetro morto remanescente.
- [x] **AC4** — A validação permanece **estrita**: um `req:` contendo apenas basename continua
      produzindo a violação. Coberto por teste.
- [x] **AC5** — `extractRefPath` e equivalentes **não** são alterados.
- [x] **AC6** — `trackfw validate` verde neste repositório — nenhum roadmap existente invalidado.
- [x] **AC7** — `scripts/check-artifact-parity.sh` passa; os 3 CLIs geram artefato idêntico.
- [x] **AC8** — O cenário `roadmap-acceptance-heading/*/from-req` de
      `scripts/check-gates-falsify.sh` continua passando. A nota de vault prevê que sim (o
      `assert_fails_with` casa a substring de `wip_acceptance`, não a ausência de outras
      violações), mas isso deve ser **confirmado empiricamente**, não presumido.
- [x] **AC9** — **Dois** cenários permanentes adicionados (não um), cada um com braço de
      *baseline* e de *detecção*, nos 3 CLIs: `roadmap-req-frontmatter-path/*/from-req` e
      `.../simple` (AC2b). Contador de cenários **30 → 42**. Falsificação independente por Zeus:
      revertendo o gerador Go para `filepath.Base(reqPath)`, o gate sai com **exit 1** e
      `FAIL [.../go/from-req-baseline]: ciclo limpo saiu com 1, esperava 0`.
- [x] **AC10** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não** fazer `referenceExists` resolver `ref` contra as raízes. Decisão explícita do usuário:
  a validação segue estrita. O parâmetro é removido, não implementado.
- **Não** alterar `extractRefPath` / equivalentes, nem a precedência frontmatter-sobre-corpo.
- Não alterar o comportamento de `roadmap new` **sem** `--req`: o campo `req: ""` vazio é
  correto nesse caso.
- Não alterar as mensagens de violação nem os nomes das regras (`ref_targets_exist`).
- Não reescrever roadmaps existentes em massa.
- Não relaxar `AcceptanceMarkers` nem mexer no heading de critérios (entregue no PR #96).
- Não tocar em `pypi/build/lib/` — artefato de build.
- Não adicionar devDependency nem dependência Python.

## Notas de implementação

Pontos exatos verificados em 2026-08-01:

| | Gerador `--from-req` (basename) | Gerador simples (`req: ""` vazio) | `referenceExists` | Chamadores |
|---|---|---|---|---|
| Go | `internal/generators/roadmap.go:175` | `internal/generators/roadmap.go:~95` | `internal/validator/validator.go:1491` | `:1462`, `:1478`, `:1483` |
| Node | `npm/src/generators/roadmap.js:517` | `npm/src/generators/roadmap.js:425` | `npm/src/validator/index.js:781` | `:757`, `:769`, `:773` |
| Python | `pypi/trackfw/generators/roadmap.py:337` | `pypi/trackfw/generators/roadmap.py:192` | `pypi/trackfw/validator.py:968` | `:937`, `:951`, `:957` |

**Nota sobre o AC2b:** com `req: ""`, o `extractRefPath` devolve string vazia (há um early-return
para `val == ""`), então **nenhuma** violação `ref_targets_exist` dispara — o link simplesmente
não existe, em silêncio. É um falso-negativo, complementar ao falso-positivo do `--from-req`.

Os três CLIs têm arquivos disjuntos → os MLs da Wave 1 rodam em paralelo. Mas `make parity` e
`make quality` só fecham com os três prontos, porque `check-artifact-parity.sh` compara os
artefatos gerados entre CLIs — por isso a paridade é barreira de Wave 2, não critério dos MLs
individuais.

## Linked ADR

ADR: docs/adr/ADR-2026-08-01-caminho-completo-no-campo-req-do-frontmatter-e-remocao-do-parametro-roots-morto.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/done/ROADMAP-2026-08-01-corrigir-falso-positivo-ref-targets-exist-em-roadmap-new-from-req.md
