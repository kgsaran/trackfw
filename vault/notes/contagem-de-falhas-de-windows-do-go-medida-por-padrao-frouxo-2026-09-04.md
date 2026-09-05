---
title: A contagem de falhas de Windows do Go foi medida por padrão frouxo, e eu reportei 69 onde havia 101
date: 2026-09-04
tags: [medicao, windows, ci, autoengano]
---

# A contagem de falhas de Windows do Go foi medida por padrão frouxo

## O que aconteceu

Ao extrair a contagem de falhas do job `windows-full-suites` do run `33913343975` (pós-Wave 2),
reportei **Go = 14**. A medição correta é **Go = 46**.

Consequência: reportei ao usuário que a campanha estava em **69** falhas quando estava em **101**,
e chamei a Wave 2 de "maior queda da campanha, 65 falhas" quando ela fechou **33**.

## A causa

O log do `gh run view --log` vem com **prefixo por linha**:

```
windows-full-suites\tGo — suíte completa (camada 1, AC1)\t2026-09-05T00:01:02.3379040Z --- FAIL: TestX
```

Um `grep` que ancore em `^--- FAIL` **não casa nada** — a linha não começa com `---`. E um padrão
frouxo demais casa também as linhas de subteste (`    --- FAIL:`), inflando. O padrão que discrimina
top-level de subteste **preservando o prefixo** é:

```bash
grep -cE 'Z --- FAIL' arquivo.log      # top-level: timestamp, espaco, --- FAIL
grep -cE '\-\-\- FAIL' arquivo.log     # inclui subtestes
```

A extração original devolveu 14 sem erro e **com exit 0** — número plausível, nenhum sinal de que
tinha medido a coisa errada.

## Por que isso passou

**A contagem nunca foi verificada contra uma segunda medição independente.** Eu tratei o número
como dado porque ele veio de um comando, e comando que sai 0 parece autoritativo.

O erro só apareceu porque a Wave 3 devolveu um delta absurdo (Go 14 → 45, uma *subida*), e a
reação certa foi **re-medir o run anterior com o mesmo padrão** em vez de explicar a subida. As duas
medições lado a lado, com padrão idêntico, mostraram que a base é que estava errada.

## A regra que fica

**Delta entre dois runs só é comparável se as duas pontas forem medidas pelo mesmo comando, na mesma
sessão.** Nunca comparar um número novo contra um número recordado — remedir a base.

E: quando um delta vier absurdo, a primeira hipótese é **erro de medição**, não fenômeno real.

## Arco real, medido de forma consistente (2026-09-04)

```
run           Go   Node   Py   TOTAL
33810452454   64    52   101    217
33875124523   64    52   101    217
33894183684   64    52    46    162
33903612484   53    48    33    134
33913343975   46    34    21    101
33931363032   45    34    21    100
```
