---
name: nao-narrar-despacho-antes-de-fazer
description: Nunca escrever "despachando/vou despachar" na resposta sem ter feito a chamada do Agent no mesmo turno
metadata:
  type: feedback
---

**Nunca descreva um despacho como feito antes de fazer a chamada.** Se a resposta diz "ML-X
despachado" ou "despachando agora", a chamada do Agent tem de estar **no mesmo turno**, antes do
texto.

**Why:** aconteceu **duas vezes** na sessão de 2026-08-27/28 — escrevi "ML-1B despachando" e "ML-1A
despachando" e encerrei o turno sem chamar. KG perguntou *"os agentes estão codando?"* e depois
*"vc já despachou?"*, e nas duas o trabalho estava parado sem ninguém saber. É o mesmo defeito que eu
cobro dos agentes a sessão inteira: **relatar como feito o que não foi medido**. Vindo do orquestrador
é pior, porque ninguém audita o auditor.

**How to apply:** ao terminar um ciclo de auditoria, a ordem é **(1) chamar o Agent, (2) escrever a
resposta**. Se por algum motivo o despacho não couber no turno, a resposta deve dizer **"vou despachar
no próximo passo"** — no futuro, nunca no passado. Vale para PR (`gh pr create`), commit e qualquer
ação que o usuário possa assumir concluída.

Relacionado: [[ler-a-req-nao-o-adr-vizinho]].
