---
name: sem-pr-aberto-implementacao-e-nossa
description: Oferta de contribuição não é PR — sem PR aberto, nós implementamos, e isso tem de ficar dito no ticket, não implícito
metadata:
  type: feedback
---

**Se o colaborador não tem PR já aberto, a implementação é nossa — e o ticket tem de dizer isso
explicitamente**, incluindo que a REQ de correção foi aberta e que vamos implementar.

**Why:** em 2026-09-09, no issue #298, o autor escreveu *"posso abrir PR com ele, se interessar"*. Eu
transformei a **oferta** em decisão pendente e perguntei ao KG se aceitávamos "o PR". Ele corrigiu:
*"não tem PR do Lourival"*. E eu **repeti o erro em público**, escrevendo no comentário do issue que
"a decisão sobre o PR é do KG" — perpetuando a ficção de que havia algo em revisão. KG mandou a tela
do GitHub: **Open: 0**.

🔴 A causa não foi falta de informação: foi eu redigir o comentário público a partir do meu raciocínio
anterior **sem reler a correção que ele já havia dado**.

**How to apply:**

- **Oferta ≠ PR.** Antes de tratar contribuição como pendente, confira: `gh pr list --state open`.
- Sem PR aberto ⇒ **nós implementamos**, e o ticket diz isso: *"a REQ de correção está aberta, o ML
  está escrito, e o código sai daqui"*.
- **Declinar com motivo técnico, não com silêncio.** No #298 o motivo era real e verificável: o ML
  exigia lista derivada, falsificação nas duas direções e exclusão do `help` do framework — três
  coisas que o gate do fork não tinha, **duas delas declaradas pelo próprio autor** como limites.
  Pedir o PR e depois exigir retrabalho é pior para o colaborador.
- **Creditar o desenho.** O gate do fork continua sendo referência, e dizer isso mantém o
  colaborador engajado sem criar dependência de PR externo no caminho crítico.
- 🔴 **Antes de publicar comentário em issue, reler a última correção do KG.** Erro repetido em
  público custa mais que erro em conversa — e exige retificação explícita, não edição silenciosa.

Relacionado: [[nao-narrar-despacho-antes-de-fazer]] (mesma família: afirmar estado que não existe).
