---
name: nao-trocar-de-branch-com-agente-vivo
description: Nunca trocar de branch, commitar de outra branch ou mexer no índice enquanto um subagente está editando a árvore; usar cópia durável ou worktree
metadata:
  type: feedback
---

Enquanto **qualquer subagente estiver vivo editando a árvore**, não trocar de branch, não fazer
checkout, não commitar artefatos de outra branch. Se um artefato precisa ser salvo com urgência,
copiar para fora do `/tmp` (ex.: `~/.trackfw/rascunhos/`) — zero interação com git — e commitar
depois, na branch certa.

**Why:** já aconteceu duas vezes nesta sessão. (1) Troquei de branch com um agente vivo e os
arquivos dele foram parar na branch errada. (2) Despachei um segundo agente na mesma worktree e o
Makefile foi revertido debaixo do primeiro. O KG levantou o risco espontaneamente quando ofereci
commitar o parecer do CONTRIBUTING numa outra branch — ele lembrou do incidente antes de mim.

**How to apply:** antes de qualquer operação de git que mude HEAD, índice ou árvore, verificar se há
subagente rodando. Se houver: adiar. A alternativa técnica que **não** toca na árvore atual é
`git worktree add`, mas ela tem custo — branch presa em worktree não pode ser apagada e polui a
análise de branches do protocolo de "uma branch ativa por vez" — então só usar com aprovação
explícita do KG, nunca por iniciativa própria.

Relacionado: a fila natural é sempre *auditar → commitar → KG mergeia → só então a próxima branch
vira ativa*. Ver [[usar-trackfw-branch-new]].
