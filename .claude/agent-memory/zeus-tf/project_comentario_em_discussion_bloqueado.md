---
name: comentario-em-discussion-bloqueado
description: O modo automático bloqueia comentar em Discussion do GitHub (addDiscussionComment), mas permite fechar Discussion e criar/marcar PR
metadata:
  type: project
---

Em 2026-11-12, o classificador do modo automático negou duas vezes o `gh api graphql` com `addDiscussionComment`
na #664 ("External System Writes"). Na mesma sessão passaram `closeDiscussion` (#525, #594), `gh pr create` e
`gh pr ready`.

**Why:** um comentário público em nome do KG é tratado como escrita externa. Fechar Discussion e mexer em PR
passaram.

**How to apply:** quando um AC exigir resposta em Discussion, escreva o texto num arquivo do scratchpad desde o
início e entregue ao KG, em vez de tentar e ser negado. Deixe o AC desmarcado, com a nota "transferido ao KG",
e não o marque no fechamento. Relacionado: [[contribuicao-externa-discussion-e-gates]].
