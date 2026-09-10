---
name: palavra-chave-de-fechamento-e-em-ingles
description: PR escrito em português não fecha issue — o GitHub só reconhece Closes/Fixes/Resolves; "Fecha #N" é texto comum
metadata:
  type: project
---

**O corpo do PR tem de usar palavra-chave de fechamento em INGLÊS**, mesmo com todo o resto em
português: `Closes #N`, `Fixes #N`, `Resolves #N`.

**Why:** em 2026-09-10 o PR **#312** dizia *"Fecha **#274** e **#275**"* e foi mergeado — **os dois
issues continuaram abertos**. O GitHub não reconhece "Fecha"; é texto comum. Tive de fechar à mão.

Medido no mesmo dia: dos 30 PRs mergeados mais recentes, **só 4** usavam a forma em inglês. Como
escrevemos tudo em português, a falha é sistêmica — e infla a contagem de issues abertos, que foi
exatamente a preocupação que o KG levantou ("issues antigos").

**How to apply:**

- Ao abrir PR que resolve issue: **uma linha em inglês**, sozinha, além do texto em português —
  `Closes #274` · `Closes #275` (uma por issue; `Closes #274 e #275` **não** fecha o segundo).
- Depois do merge, **conferir**: `gh issue view <N> --json state`. Não presumir que fechou.
- Se não fechou, fechar com `gh issue close` **e comentário com a evidência** — não só o número do PR.
- 🔴 **O gate existe e é bom — mas markdown o dribla.** `scripts/check-pr-closing-keyword.sh`
  recusa `Fecha #N`, e **deixa passar `Fecha **#N**`**: o `**` quebra a adjacência que ele procura.
  Foi assim que o #312 passou verde. Medido em 2026-09-10 e roteado como ML no roadmap do issue
  **#258**, que governa esse gate.

Relacionado: [[sem-pr-aberto-implementacao-e-nossa]] — mesma família: o estado do GitHub não é o que
eu presumo, é o que eu confiro.
