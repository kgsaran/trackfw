---
title: roadmap com CRLF diverge o Node do Go e do Python no barrier — `.` do JS exclui \r, o do RE2 inclui
tags: [barrier, crlf, windows, paridade, gotcha]
date: 2026-08-29
related: [[barrier-so-casa-cabecalho-de-aceite-em-portugues-2026-08-29]]
---

## Sintoma

Roadmap idêntico, salvo com terminadores **CRLF**, mesmo comando:

```
go     ✓ mls_complete: passed
py     ✓ mls_complete: passed
node   ✗ mls_complete: blocked — "ML-1A: not complete (status: missing)"
```

O `barrier` do Node diz que o status está **faltando** num ML onde ele está preenchido.

## Causa Raiz — três primitivas, três comportamentos

Os 3 runtimes dividem o roadmap em linhas com `split("\n")`, o que deixa um `\r` no fim de cada
linha num arquivo CRLF. O que acontece depois **diverge por acidente de linguagem**:

| runtime | por que passa ou falha |
|---|---|
| **Node** | `.` numa regex JS **exclui** `\r` — é `LineTerminator` na spec do ECMAScript. `/^\*\*Status:\*\*(.*)$/` simplesmente **não casa** `"**Status:** ✅ Concluído\r"`. **É aqui que o defeito mora.** |
| **Go** | `.` no RE2 **inclui** `\r` (exclui só `\n`), e todo consumo passa por `strings.TrimSpace`, que trata `\r` como espaço. Passa por dois motivos independentes. |
| **Python** | `open(path, "r")` faz universal newlines por padrão (`newline=None`): `\r\n` vira `\n` **antes** de o parser existir. O `\r` nunca chega. |

Nenhum dos três está errado isoladamente. A divergência é emergente — e invisível em qualquer
máquina que só produza LF.

## Por que importa fora do nosso repositório

A issue #216 mediu que os **geradores Python escrevem CRLF no Windows** (42 `open(...,'w')` e 20
`write_text`, nenhum com `newline=`). Somando os dois defeitos: **todo roadmap criado no Windows
pelo CLI Python fica ilegível para o CLI Node.** O usuário vê o `barrier` acusar status ausente em
MLs preenchidos, sem nenhuma pista do porquê.

## Correção

Normalizar **uma vez, na fronteira de entrada** — onde o arquivo é dividido em linhas —, não
remendando regex por regex: são nove marcadores por runtime, e o próximo que alguém acrescentar
nasceria com o bug de novo. `splitRoadmapLines` (Go/Node), `_split_roadmap_lines` (Python).

## Armadilha para quem for testar isso

**A normalização só é load-bearing no Node.** Em Go e Python ela é, hoje, um no-op: o `TrimSpace` e
o universal-newlines já absorvem o `\r` antes de qualquer comparação. Nenhuma fixture CRLF consegue
distinguir "chamou a função de fronteira" de "não chamou" nesses dois runtimes.

O agente que implementou **não** falsificou isso com asserção de call-site ou mock — documentou a
lacuna no doc-comment de `TestSplitRoadmapLines_StripsTrailingCROnlyAtBoundary`. Está certo: um teste
que prova que uma função foi chamada não prova que ela faz diferença, e foi assim que o ML-2G do
ciclo anterior escapou da auditoria.

Existe um jeito de tornar load-bearing no Python — `open(path, "r", newline="")` — e ele foi
**rejeitado**: regrediria o tratamento de arquivos com CR solto (Mac antigo), que hoje o Python
ganha de graça, sem nenhum defeito forçando a mudança.

## Assimetria pré-existente, registrada e fora de escopo

Arquivo com **CR solto** (sem `\n`) é tratado pelo Python (universal newlines) e **não** por Go e
Node — que, dividindo só por `\n`, veem o arquivo inteiro como uma linha. Não há defeito reportado
que force isso; fica documentado.
