---
name: falsificacao-nas-duas-direcoes
description: Todo microlote de gate/QA fecha com falsificação nas duas direções e saída real colada; remédio de parecer que não se sustenta na medição volta como medição, não como patch silencioso
metadata:
  type: feedback
---

Fechar microlote de gate exige, além de build/testes verdes: falsificação **nas duas direções**
(mutação → reprova; árvore íntegra → passa) com a **saída real** colada no relatório, e
re-falsificação **por execução** de toda guarda de vacuidade que o microlote encostou — "continua
funcionando" presumido não conta.

**Why:** o `trackfw_architect` despacha microlotes corretivos justamente quando um gate devolveu
`exit 0` sobre a regressão que existe para pegar (*fail-open*). Um `make quality` verde não
contradiz um fail-open — ele o confirma. Por isso o critério de aceite é a saída da mutação, não o
veredito da árvore. E o handoff pede explicitamente: **se um remédio proposto por um parecer não se
sustentar quando medido, reportar com a medição** — o arquiteto prefere reabrir a discussão a
receber um remédio que não conserta.

**How to apply:** falsificar em **cópia de sandbox** (nunca mutar a árvore e restaurar — restaurar é
o passo que falha em silêncio); um gate que deriva a raiz de `BASH_SOURCE` se auto-enraíza na cópia.
Residual que sobra e é *fail-closed* se **reporta e documenta** (nota de vault + anotação de
contrato), não se corrige de improviso: mudança extra no bloco que os auditores já revisaram faz a
barreira recomeçar. Ver [[fronteira-de-escrita-qa-sem-git]].
