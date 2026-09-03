---
status: Open
date: 2026-09-02
author: "lourivalgarciajunior"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-02-o-teste-do-gitattributes-verifica-conteudo-em-vez-de-fixar-o-arquivo.md"
---

# REQ: o teste do `.gitattributes` fixa o arquivo inteiro, e aí o repositório não pode ter regra própria

> Date: 2026-09-02 | Status: Open

## Motivation

Os três runtimes afirmam **igualdade do arquivo inteiro** contra o bloco gerado:

```go
versioned, _ := os.ReadFile(filepath.Join("..", "..", ".gitattributes"))
if string(versioned) != gitAttributesBlock { ... }
```

```javascript
assert.equal(versioned, GITATTRIBUTES_BLOCK)
```

```python
assert _read(os.path.join(REPO_ROOT, ".gitattributes")) == GITATTRIBUTES_BLOCK
```

A intenção está certa e está escrita no comentário: *"o arquivo deste repositório e o que o `init`
gera não podem divergir"*. O predicado é que é forte demais — ele não afirma que o bloco está lá,
afirma que **não há mais nada no arquivo**.

Consequência: o repositório do trackfw não pode acrescentar nenhuma regra de `.gitattributes`
própria. Nem `* text=auto`, nem `*.go text eol=lf` — que é exatamente o que um projeto Go
desenvolvido no Windows precisa.

## A evidência, medida

Este é um achado de consumidor: bati nele horas depois do #251, ao trazer a `main` para um fork que
já tinha `.gitattributes` com normalização de fim de linha.

Medido em clone limpo com `core.autocrlf=true`, apagando `internal/` e refazendo `git checkout`:

| `.gitattributes` da raiz | arquivos `.go` com CRLF |
|---|---|
| bloco do `init` **+** as regras de EOL | **0** de 213 |
| **só** o bloco do `init` | **213** de 213 |

Sem as regras de EOL, todo `.go` do repositório vem com CRLF no working copy, e `gofmt -l` passa a
acusar arquivo que não tem desvio nenhum — poluindo qualquer verificação de formatação e mascarando
desvio de verdade.

O teste, como está, **proíbe a única correção desse problema.**

## O produto está certo

Medido: `trackfw init` num projeto que já tem `.gitattributes` **anexa** e preserva o que existia.
O caminho de APPEND está correto e coberto. É só o teste de auto-consistência que assume que o
arquivo da raiz contém apenas o bloco.

## Acceptance Criteria

- [ ] **AC1** — Os três testes passam a verificar **contenção** em vez de igualdade: o arquivo da
      raiz precisa **conter** o bloco gerado, byte a byte.
- [ ] **AC2** — 🔴 **A falsificação continua valendo.** Se o bloco derivar do arquivo da raiz, os
      três testes ainda reprovam. É isso que o teste existe para pegar, e não pode enfraquecer.
- [ ] **AC3** — Um repositório com regra própria antes do bloco passa nos três.
- [ ] **AC4** — Paridade: a mesma mudança nos três runtimes.

## Negative Scope

**Não muda o gerador nem o bloco.** O produto está correto; só o predicado do teste muda.

**Não acrescenta regra de EOL ao `.gitattributes` de vocês.** Se vale para o repositório de vocês é
decisão de vocês — a medição está aqui, mas a REQ é sobre o teste que **impede** a decisão, não sobre
a decisão.

## Linked ADR
<!-- Correção de predicado de teste; sem decisão nova a registrar. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/wip/ROADMAP-2026-09-02-o-teste-do-gitattributes-verifica-conteudo-em-vez-de-fixar-o-arquivo.md
