---
status: Open
date: 2026-09-03
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-03-declarar-eol-lf-para-os-fontes-na-raiz-do-gitattributes.md"
---

# REQ: A raiz do `.gitattributes` não declara `eol` para os fontes, e 217 arquivos Go chegam em CRLF no Windows

> Date: 2026-09-03 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

`core.autocrlf=true` é o default do runner Windows do GitHub. Sem `eol` declarado, **todo** arquivo
de fonte chega ao checkout com `\r\n`. Medido agora, em clone local com `autocrlf=true`:

```
arquivos .go com CRLF no checkout:  217
total de arquivos .go:              217
```

**217 de 217.** O `gofmt -l` passa a acusar arquivo sem desvio nenhum — o PR #254 do reporter mediu
`0 de 213 com a regra, 213 de 213 sem ela`.

### Por que só agora

O **PR #254** (mergeado, `c9438bd`) removeu o impedimento: os 3 testes de `gitattributes` passaram de
**igualdade** para **contenção**, porque o `init` **anexa** o bloco a um arquivo existente — exigir o
arquivo inteiro era um teste mais estrito que a semântica que ele testa.

Antes dele, a raiz era **proibida de carregar a regra de que ela própria precisava**: a barreira do
ML-1C mediu isso e concluiu *"sem saída dentro do repositório"*. A saída existia e era outra — o
reporter a encontrou.

🔴 **Mas o #254 só removeu o impedimento; não acrescentou a linha.** Verificado na `main` após o
merge: o `.gitattributes` da raiz tem **apenas** a regra `.trackfw-log merge=union`. **O efeito nos
217 arquivos não está acontecendo.**

### Por que isto importa para a contagem de Windows

A Wave 0 (`REQ-2026-09-03-setenta-e-tres-...`) desmascarou bloqueios de medição para que a contagem
de falhas de Windows passasse a significar algo. **Esta é a maior redução de contagem ainda
disponível, e custa uma linha** — e enquanto ela não existir, uma fatia das 86 falhas Go é ruído de
fim de linha, não defeito.

## Acceptance Criteria

- [ ] **AC1** — A raiz declara `eol=lf` para os fontes dos 3 stacks (`*.go`, `*.js`, `*.py`) e para
      os artefatos que testes leem como texto. 🔴 **Enumerar por medição**, não por intuição: rodar
      o clone com `autocrlf=true` e listar o que de fato chega em CRLF.
- [ ] **AC2** — 🔴 **Falsificação nas duas direções:** com a regra, 0 arquivos em CRLF no clone
      `autocrlf=true`; sem ela, 217. Medido, não presumido.
- [ ] **AC3** — 🔴 **Controle — os 3 testes de contenção do #254 continuam passando.** Eles comparam
      a raiz com uma constante de **produto**; a regra nova entra **fora** do bloco gerado pelo
      `init`, e a contenção tem de continuar satisfeita nos 3 runtimes.
- [ ] **AC4** — 🔴 **A metade de PRODUTO do CRLF permanece visível.** O parser de frontmatter é cego
      a CRLF e emitiu frontmatter duplicado em `TestRenderOpenCodeAgent`. Declarar `eol` sobre os
      **assets que o produto processa** esconderia o defeito — foi exatamente por isso que o ML-1C
      **removeu** o pin que já tinha feito. **Enumerar o que fica de fora, e por quê.**
- [ ] **AC5** — `merge=union` do `.trackfw-log` intacto, confirmado por `git check-attr`.
- [ ] **AC6** — Renormalização não gera ruído de diff: `git ls-files --eol` mostra o índice já em
      `lf`, então a regra não deve reescrever nada. Verificar.
- [ ] **AC7** — `make quality` verde e `trackfw validate` exit 0.

## Negative Scope

- 🔴 **Não** declarar `eol` sobre `*/integrations/assets/**` nem sobre
  `internal/integrations/testdata/*.golden.*`. São entrada do produto e do parser cego a CRLF —
  esconderiam o defeito que a ADR de CRLF vai decidir. Medido pelo ML-1C: fixar só o lado esperado
  **cura zero** e apaga a evidência.
- **Não** tocar no bloco `GITATTRIBUTES_BLOCK` dos 3 geradores. A regra é deste repositório, não do
  produto — projeto adotante decide o `eol` dele.
- **Não** alterar `scripts/testdata/roadmap-barrier-corpus-snapshot/` — entrada do barrier.
- **Não** rodar `git add --renormalize` sem medir o AC6 primeiro.

## Linked ADR
<!-- Configuração de repositório; a decisão de arquitetura pendente é outra: se o PARSER deve
     tolerar CRLF (ADR própria, ainda não escrita). Esta REQ não a antecipa. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-03-declarar-eol-lf-para-os-fontes-na-raiz-do-gitattributes.md
