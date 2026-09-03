---
name: mensagem-de-log-prova-o-que-ja-rodou
description: Usar a mensagem que apareceu no log como marcador de progresso — ela falsifica hipóteses sobre passos anteriores, mesmo sem acesso ao ambiente (ex. Windows)
metadata:
  type: feedback
---

Quando não dá para rodar no ambiente que falhou (Windows sem runner local), **a mensagem que
apareceu no log é evidência sobre tudo que rodou antes dela**. Se a hipótese diz que o passo N
falhou, mas o log traz uma mensagem emitida no passo N+3, a hipótese está falsificada — sem precisar
reproduzir nada.

**Why:** no ML-1E de 2026-09-03 a nota de vault afirmava que o `fs.symlinkSync` da fixture "exige
Developer Mode para ser criado" no Windows. Mas a mensagem observada, `git ... exited with null`, é
emitida por `npm/src/ship/runner.js` — ou seja, **dentro do produto já spawnado**. Se o symlink
tivesse sido negado, o teste teria morrido antes, e essa mensagem não existiria no log. O bloqueio
real era só o nome sem extensão (PATHEXT). A distinção mudou o remendo: symlink continua sendo a
forma primária no Windows (o processo roda com o caminho do alvo, então o wrapper do Git for Windows
acha sua instalação), enquanto a cópia — que a hipótese velha exigiria — poderia quebrá-lo.

**How to apply:** antes de aceitar "o passo X falhou no CI", localizar no código **quem emite** cada
string do log e ordenar as emissões. O que veio depois prova o que veio antes. Vale também ao
escrever fixture: se um destino colocado puder falhar de um jeito indistinguível do defeito
original, acrescentar uma sonda que nomeie o mecanismo, para o log continuar discriminante.
Relacionado: [[diagnostico-herdado-pode-estar-vencido]], [[falsificacao-nas-duas-direcoes]].
