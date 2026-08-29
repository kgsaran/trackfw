---
title: cada linha do bloco `**Gates da wave:**` é um comando INDEPENDENTE — script multilinha não funciona
tags: [barrier, gate, roadmap, gotcha]
date: 2026-08-29
related: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## O contrato real

`parseGates` (`internal/commands/barrier.go:623-633`) coleta **cada linha não vazia e não comentada**
do bloco ` ```bash ` como um **comando separado**, e executa cada um em seu próprio processo:

```go
line := strings.TrimSpace(lines[k])
if line != "" && !strings.HasPrefix(line, "#") {
    cmds = append(cmds, line)
}
```

**Não é um script.** Não há estado compartilhado entre as linhas.

## O que isso quebra, e passa despercebido

```bash
set -eu                                   # roda sozinho, não afeta nada depois
esperado="a
b"                                        # atribuição multilinha: quebra
n=$(grep -c foo arquivo)                  # $n morre com o processo
[ "$n" -eq 8 ] || { echo erro >&2; exit 1; }   # $n vazio aqui → erro de sintaxe do test
```

E o caso que me pegou:

```bash
grep -q "padrão" arquivo && { echo "achou" >&2; exit 1; }
```

Quando o `grep` **não acha** — que é o caso bom —, a lista `A && B` devolve o status do `grep`, que é
**1**. O `barrier` registra `exit 1` e **bloqueia a wave**. O gate reprova exatamente quando deveria
passar.

Rodado à mão como script (`sh -c 'set -eu; grep ... && {...}; echo ok'`) o mesmo texto **passa**,
porque aí o `&&` é uma condição no meio de uma lista e o `echo` final devolve 0. É por isso que o
defeito sobrevive à verificação manual: *o texto funciona como script e falha como linhas soltas.*

## A forma correta

Uma linha, autocontida, cujo **código de saída** é a asserção:

```bash
! grep -q "git show" scripts/check-roadmap-barrier-contract.sh
```

Sai 0 quando o padrão está ausente, 1 quando presente. Sem `set -e`, sem variável, sem bloco.

Se a verificação for grande demais para uma linha, **ponha num script** em `scripts/` e chame o
script — que é o que os gates de wave maduros deste projeto já fazem
(`bash scripts/check-...sh`).

## Por que isso é armadilha de verdade

O formato **aceita silenciosamente** um bloco multilinha que não pode funcionar. Não há erro de
parsing, não há aviso: as linhas viram comandos, alguns dão 0 por acidente, e o autor conclui que o
gate funciona. Escrevi gates assim o dia inteiro em três roadmaps antes de perceber, e só percebi
porque um deles reprovou **na direção errada**.

Vale considerar uma regra de `validate` ou um aviso do `barrier` para bloco de gate que contenha
`set -e`, atribuição de variável ou `if`/`{` — sinais de que o autor achava que estava escrevendo um
script.
