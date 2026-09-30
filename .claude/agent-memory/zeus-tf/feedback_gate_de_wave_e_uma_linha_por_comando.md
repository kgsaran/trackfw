---
name: gate-de-wave-e-uma-linha-por-comando
description: o barrier executa cada linha do bloco "Gates da wave" como sh -c separado — gate multilinha que passa com bash reprova sob o barrier real; testar gate com o barrier, não com bash
metadata:
  type: feedback
---

Gate de wave é **uma linha por comando**: o `ParseGates` executa cada linha como `sh -c` isolado.
`n=$(python3 -c "` em várias linhas vira N comandos quebrados; `export` numa linha não chega à seguinte.

**Why:** em 2026-09-30 (#485) escrevi o gate da Wave 0 multilinha, testei extraindo o bloco e rodando
com `bash` — passou nos 3 braços. Sob o `trackfw barrier` real reprovava com 13 falhas. O mesmo erro
deixou o fusível de um teste do Hades em linha separada e uma recursão correu ~120 s.

**How to apply:** ao escrever gate, conferir com o **barrier real** (`--trust-local-gates`, binário
atual), nunca só com `bash`. Lógica longa → um `python3 -c "...;..."` de uma linha. O gate da Wave 0 do
#476 (branch `fix/cerca-nao-terminada-mascara-em-silencio`) ainda tem esse defeito. Ver
[[medir-com-a-regra-nao-com-grep]] e [[o-instrumento-mente]].
