---
status: done
date: 2026-09-02
req: "docs/req/REQ-2026-09-02-tripwire-de-disco-trunca-o-corpus-congelado-e-a-ac10-acusa-reclassificacao-que-nao-houve.md"
---

# ROADMAP: separar a tripwire de disco do contrato da AC10

> Date: 2026-09-02 | Status: done

REQ: docs/req/REQ-2026-09-02-tripwire-de-disco-trunca-o-corpus-congelado-e-a-ac10-acusa-reclassificacao-que-nao-houve.md
ADR:

## ML-1A — Remover o `continue` que tira o arquivo do corpus congelado

**Status:** ✅ Concluído

Uma linha. O `continue` que segue o registro em `MISSING_FROM_DISK` fazia o arquivo sair do scan —
e com ele saíam a contagem, as waves e as linhas de veredito que compõem o hash da AC10.

O veredito **já** vinha dos bytes do snapshot; o comentário logo abaixo do `continue` diz isso. Só a
decisão de *entrar no scan* estava sujeita ao disco. Nenhum dado pinado é regenerado.

## ML-1B — Falsificação nas duas direções

**Status:** ✅ Concluído

**Direção do defeito — antes.** Apagando **um** roadmap de `docs/roadmaps/done/` numa árvore limpa
do `main`:

```
FAIL [corpus/basename-missing-from-disk]           ← legítima
corpus: files=143 waves=426 exit2=14 lines=1476
FAIL [corpus/files-count]                            143, pinado 144
FAIL [corpus/waves-count]                            426, pinado 432
FAIL [corpus/mls-complete-verdict-counts]            evidence=627, pinado 639
FAIL [corpus/acceptance-evidence-verdict-counts]     evidence=306, pinado 314
FAIL [corpus/non-reclassification]                   "corpus reclassificado: hash mudou"
```

Seis falhas: **uma legítima e cinco falsas**. A última afirma reclassificação que não houve.

**Direção da correção — depois.** Mesmo roadmap apagado, mesmo gate:

```
FAIL [corpus/basename-missing-from-disk]   <- a legitima, e agora a UNICA
corpus (snapshot=...): files=144 waves=432 exit2=14 lines=1500
OK   [corpus/files-count]
OK   [corpus/waves-count]
OK   [corpus/exit2-count]
OK   [corpus/mls-complete-verdict-counts]
OK   [corpus/acceptance-evidence-verdict-counts]
OK   [corpus/non-reclassification]
```

**Seis falhas viraram uma.** As contagens voltam a bater com os pinos — `files=144`, `waves=432`,
`exit2=14` — e o hash da tabela de vereditos volta a fechar. A tripwire continua reprovando, que é
o que ela deve fazer; o que parou foi ela arrastar a AC10 junto.

**Controle da AC10** — ela precisa continuar acendendo pelo motivo certo, ou a correção teria
desligado o contrato em vez de consertá-lo. Plantei uma reclassificação **real** no snapshot: um ML
de `✅ Concluído` para `⬜ Pendente`, num único arquivo. Com o roadmap de volta no disco:

```
OK   [corpus/basename-missing-from-disk]        <- nenhum ausente, a tripwire cala
corpus: files=144 waves=432 exit2=14 lines=1500  <- estrutura intacta
OK   [corpus/files-count]
OK   [corpus/waves-count]
OK   [corpus/exit2-count]
FAIL [corpus/mls-complete-verdict-counts]: evidence=638 failure=114, pinado 639/113
OK   [corpus/acceptance-evidence-verdict-counts]
FAIL [corpus/non-reclassification]: hash da tabela de vereditos mudou
```

A AC10 acende, e acende **com precisão**: exatamente **um** ML migra de `evidence` para `failure`
(639/113 → 638/114), que é a única coisa que mudei. As contagens estruturais seguem intactas,
porque nenhum arquivo saiu do corpus. É a diferença entre reclassificar e truncar — e agora o gate
distingue as duas.

## ML-1C — O que esta correção NÃO resolve, dito na frente

**Status:** ✅ Concluído

A tripwire compara um **fixture de teste** com o conteúdo **vivo** de `docs/roadmaps/**`. Por isso
ela não pode passar em nenhum fork ou downstream cuja árvore de governança seja legitimamente outra
— e esse é o caso que me trouxe aqui.

É a **segunda vez no mesmo dia** que encontro esta forma: o teste do `.gitattributes` (issue #253)
fixa o arquivo inteiro da raiz, e este fixa o corpus contra a árvore viva. Nos dois, uma asserção de
auto-consistência assume o conteúdo do repositório que a escreveu.

Se a tripwire deve ser fatal, ou virar `not-evaluated` quando a árvore não é a do corpus — o mesmo
vocabulário que o `doctor` já usa —, é decisão de quem mantém. Fica registrado, não decidido.
