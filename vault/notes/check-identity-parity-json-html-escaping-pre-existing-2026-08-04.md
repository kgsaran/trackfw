---
title: check-identity-parity.sh falha pré-existente na branch feat/comando-trackfw-branch-new — JSON HTML-escaping do Go diverge de Node/Python em 6 targets
date: 2026-08-04
tags: [identity, json, go, nodejs, python, parity, pre-existing]
---

## Contexto

Rodando `make quality` na branch `feat/comando-trackfw-branch-new-para-bloquear-criacao-de-branch-sem-req-roadmap-em-wip`
(ML-3A: documentar `trackfw branch new` + gate de paridade), `scripts/check-identity-parity.sh`
falha com `Identity parity: 6 check(s) failed`, **antes** de `scripts/check-artifact-parity.sh` ser
alcançado. Não é causado pelo trabalho desta sessão — reproduzido em `git stash` (árvore idêntica ao
HEAD da branch, `d37ff63`, sem nenhuma das mudanças de ML-3A) com o mesmo resultado exato.

## Sintoma

6 falhas, todas do mesmo tipo, em 3 targets (`amazonq`, `antigravity=legacy-cli`, `kiro=cli`) × 2
modos (`with-identity`, `no-identity`):

```
Identity parity [with-identity] target 'amazonq': artifacts diverge from the Go CLI in: node python
```

O diff do conteúdo do prompt mostra que a saída do Go contém `<slug>` onde Node.js e
Python emitem `<slug>` (o texto literal, sem escape). Único ponto de divergência — o resto do prompt
é byte-idêntico.

## Causa provável (não confirmada por leitura de código nesta sessão — fora de escopo do ML-3A)

`encoding/json.Marshal` do Go faz HTML-escaping por padrão (`<`, `>`, `&` → `<`, `>`,
`&`), a menos que o encoder use `SetEscapeHTML(false)`. Node.js (`JSON.stringify`) e Python
(`json.dumps`) não escapam esses caracteres por padrão. O texto de origem do prompt contém o
placeholder literal `<slug>` (visível no texto do `Dispatch contract` do papel Architect: "sempre
`<slug>-tf`, onde `<slug>` depende..."), o que expõe a divergência.

## Por que não foi corrigido aqui

Fora do escopo do ML-3A (documentar `trackfw branch new` + gate de paridade). Não é uma regressão
introduzida por esta sessão — confirmado empiricamente reproduzindo em árvore limpa (stash).
Reportado ao orquestrador para triagem/roadmap próprio.

## Como confirmar rapidamente na próxima sessão

```bash
GOCACHE=/tmp/trackfw-go-cache go build -o bin/trackfw ./cmd/trackfw
GO_BIN=bin/trackfw scripts/check-identity-parity.sh
```

Se a mensagem "Identity parity: 6 check(s) failed" persistir com os mesmos 3 targets, o bug ainda
não foi corrigido — não investigar do zero, começar por
`internal/identity` (serialização Go) vs `npm/src/identity.js` / `pypi/trackfw/identity.py`, e
procurar `SetEscapeHTML` (Go) / equivalente de escaping em Node/Python.
