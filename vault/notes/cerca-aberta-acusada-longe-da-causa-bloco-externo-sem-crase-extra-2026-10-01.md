# A cerca "aberta" acusada fica longe da causa: bloco externo transcreve outra cerca

**Data:** 2026-10-01 · **Contexto:** #476, ML-1A (acervo)

## Sintoma
`FenceMaskCheck` acusa cerca aberta na linha 460 (ou 1044) — perto do **fim** do arquivo. O
reflexo é pôr um fechador ali perto, e a contagem fecha.

## Causa real
Muito antes, um bloco ```` ``` ```` **transcreve** um exemplo que contém outra cerca (```` ```bash ````
dentro de uma saída de terminal, ou um roadmap de sonda com cercas). Pela regra CommonMark, o
```` ``` ```` interno **fecha o bloco externo**; dali em diante **todo par fica deslocado em um**, e a
última cerca do arquivo sobra "aberta".

Arquivo 08-22: o defeito estava na **223** (acusado na 460). Arquivo 08-29: na **948** (acusado na
1044).

## Por que o fechador ingênuo é pior
Ele zera o contador e **mantém o deslocamento**: seções inteiras de prosa continuam lidas como código,
e um ```` ``` ```` órfão aparece na renderização. O gate de acervo passa e o documento continua errado.

## Remédio
Cercar o bloco externo com **mais crases** que o interno (```` ```` ````). Para achar o par certo: liste
os pares que o parser forma e procure o primeiro em que um marcador **com info string** (```` ```bash ````,
```` ```trailing ````) aparece dentro de um bloco aberto — ali o autor quis aninhar.

Ferramenta: o script de pares em Python do ML-1A (imprime "interno ignorado") ou `FenceMask` linha a linha.
