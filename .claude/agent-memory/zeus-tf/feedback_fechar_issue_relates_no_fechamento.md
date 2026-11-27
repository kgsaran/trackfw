---
name: fechar-issue-relates-no-fechamento
description: PR com "Relates to #N" não fecha a issue; o fechamento da REQ precisa fechar a issue explicitamente (esqueci #704/#705)
metadata:
  type: feedback
---

Quando o PR de correção cita a issue com "Relates to" (sem palavra-chave de fechamento), a issue fica aberta depois do merge. O PR de fechamento da REQ/release deve incluir `gh issue close N --comment ...` (com texto aprovado pelo KG) ou usar `Closes #N`.

**Why:** 2026-11-26 — #704 e #705 ficaram abertas depois da 11.9.1 publicada; o KG perguntou se ainda estávamos tratando. Eu tinha prometido fechar no fechamento da REQ e esqueci.

**How to apply:** no checklist de fechamento pós-merge, listar as issues citadas no PR e fechá-las (texto curto mostrado ao KG antes). Ver [[corpo-de-pr-sem-fecha]].
