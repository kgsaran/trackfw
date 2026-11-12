---
name: comentario-em-discussion-bloqueado
description: O modo automático bloqueia comentar em Discussion (addDiscussionComment) sem aprovação explícita do KG; com o texto lido e "pode publicar", passa
metadata:
  type: project
---

Em 2026-11-12, o classificador do modo automático negou duas vezes o `gh api graphql` com `addDiscussionComment`
na #664 ("External System Writes"). Na mesma sessão passaram `closeDiscussion` (#525, #594), `gh pr create` e
`gh pr ready`.

**Why:** um comentário público em nome do KG é tratado como escrita externa. Fechar Discussion e mexer em PR
passaram.

Depois que o KG leu o texto e disse "pode publicar", a mesma chamada passou
(#664, discussioncomment-19570806).

**How to apply:** quando um AC exigir resposta em Discussion, escreva o texto num arquivo do scratchpad, mostre
ao KG e publique só depois do "pode publicar" dele, em vez de tentar antes e ser negado. Até a publicação, o AC fica desmarcado. Relacionado: [[contribuicao-externa-discussion-e-gates]].
