---
name: branch-new-worktree-add-no-b-empilha-HEAD-2026-10-18
description: git worktree add <path> sem -b cria branch nomeada pelo último componente de <path> a partir do HEAD — mesmo bug de empilhamento de branch new, caminho diferente; guard não bloqueia
metadata:
  type: project
---

## Achado

`git worktree add <path>` sem os flags `-b`/`-B`/`--detach`/`--orphan` e sem `<commit-ish>` cria
automaticamente uma branch nomeada pelo último componente de `<path>`, a partir do HEAD atual.

Medido em 2026-10-18 (hades-tf, Wave 0 REQ-2026-09-12):

```
git worktree add /tmp/wt    → cria branch "wt" a partir do HEAD, exit 0
```

O guard atual bloqueia `git worktree add -b <branch> <path>` (reason `reasonWorktreeAddB`), mas
**não** bloqueia a forma sem `-b` (exit 0 medido no binário v9.8.0).

**Por que importa:** é o mesmo bug de empilhamento de `branch new` (`internal/commands/branch.go:141`)
chegando por outro caminho — a branch nasce empilhada sobre o HEAD sem fetch nem start point, em
silêncio.

**Status:** não corrigido. Entra no roadmap
`ROADMAP-2026-09-12-branch-new-cria-da-base-correta-e-o-guard-fecha-o-checkout-cru.md` como ML-1C
pela Regra Dura de Causa Raiz (mesma causa → mesmo roadmap).

**Fix:** adicionar regra no guard para bloquear `git worktree add <path>` quando não há `-b`/`-B`/
`--detach`/`--orphan` e o segundo positional (o `<commit-ish>`) está ausente ou é HEAD implícito.
A forma `git worktree add <path> origin/main` (com `<commit-ish>` explícito) produz detached HEAD
e é segura — deve passar.

**Why:** mesma causa que o PR #333 (branch empilhada sobre commits alheios); o worktree é um
caminho frequente para abrir uma segunda frente de trabalho rápida.

**How to apply:** ao auditar `trackfw guard git-branch` para uma nova regra de criação de branch,
verificar também a forma `worktree add` sem `-b`.
