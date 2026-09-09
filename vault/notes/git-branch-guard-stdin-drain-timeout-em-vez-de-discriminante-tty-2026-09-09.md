---
title: guard drenava stdin sem limite atrás de um discriminante errado (`-t 0`) — trocado por `read -t <segundos> -d ''`, builtin do bash, sem depender de `timeout(1)`
tags: [git-branch-guard, stdin, hooks, portabilidade, bash, gotcha]
date: 2026-09-09
related: [[git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09]]
---

## Sintoma

`scripts/trackfw-git-branch-guard.sh` (e as 6 cópias-fonte: 3 geradores + 3 referências do
`validate`) drenavam a stdin ANTES de qualquer saída antecipada com:

```sh
[ -t 0 ] || _TRACKFW_STDIN=$(cat 2>/dev/null || true)
```

Isso pendurou o `make quality` por 1h05 (até ser morto) quando o gate de schema (ML-2A da
`ROADMAP-2026-09-09-guard-emite-hookspecificoutput-e-a-razao-chega-ao-modelo-nos-3-clis.md`)
invocou o guard sem `</dev/null` em 3 sítios.

## Causa raiz

`[ -t 0 ]` é o discriminante **errado**: separa "terminal interativo" de "pipe", não "vai
receber EOF" de "não vai". Sob `make`, a stdin não é terminal e nunca fecha ⇒ `cat` bloqueia
para sempre — mas **qualquer** chamador não-tty que segure stdin aberta (ex.: um runtime de
hook de agente que não fecha o descritor) reproduz o mesmo travamento, fora do contexto de
`make`. É por isso que o "arm do controle" (rodar o gate isolado) passava: a stdin herdada
tinha EOF, e só o ambiente real do `make` expunha o defeito.

## Fix — mecanismo escolhido e por quê

```sh
_TRACKFW_STDIN=""
IFS= read -r -t 2 -d '' _TRACKFW_STDIN || true
```

- `read -t <segundos> -d ''` é **builtin do bash desde a 3.0** — não é `timeout(1)` do
  coreutils, que **não existe** no macOS base nem no MSYS2 mínimo (Git-Bash do Windows).
- `-d ''` faz o `read` tentar ler até o EOF real (delimitador NUL, que nunca aparece em
  payload de hook JSON).
- `-t <segundos>` interrompe a espera no orçamento. Medido em bash 3.2 (padrão do macOS) e
  bash 5.3 (Homebrew/Linux) contra um FIFO aberto que nunca fecha: **o `read` preserva no VAR
  qualquer prefixo já lido antes do timeout** — não há perda do que chegou, só do que nunca
  chegou. Isso é o que permite ao guard continuar decidindo por `$_TRACKFW_STDIN` quando o
  payload chega antes do limite, e cair para `$*`/vazio quando não chega.
- Sob `set -e`, o `read` retorna não-zero tanto por timeout quanto por EOF-sem-NUL — **sempre**
  precisa do `|| true` para não abortar o script.
- Orçamento de 2s: folga generosa para o payload pequeno de um hook mesmo sob scheduler
  carregado (CI, `make quality` paralelo), e curto o bastante para o travamento continuar
  perceptível em vez de indefinido.
- Estourar o orçamento **não libera o bloqueio**: o passo 1 do guard já prefere `$*` quando há
  argumento posicional; sem argumento, cai para o que foi lido até o limite. `exit 2` e o
  fail-closed são idênticos nos dois casos — falsificado nas duas direções (payload+EOF decide
  pelo payload; FIFO sem EOF desiste em ~2s e decide por `$*`, sem travar).

## Por que isso importa para quem mexer em hooks de novo

Qualquer script que drena stdin como proteção de EPIPE (ver
[[git-branch-guard-schema-decision-block-rejeitado-pelo-claude-code-2026-09-09]] — o próprio
dreno nasceu para evitar EPIPE em quem escreve o payload) **precisa de limite de tempo**, nunca
de um discriminante por tipo de descritor. `-t 0` (ou qualquer teste de "é terminal?") não
prova "vai fechar" — só prova "é interativo agora". Runtimes de agente que controlamos hoje
fecham o descritor; os que não controlamos (7 CLIs suportados) podem não fechar, e o sintoma
seria "o agente congelou", sem nenhuma pista apontando para o guard.
