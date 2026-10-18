---
name: feedback_branch_new_quebra_testes_existentes_sem_origin
description: Quando branch new passa a exigir origin, testes existentes que usam makeE2ERepo sem origin quebram no arm de controle (não no negativo, que falha antes no gate de governança)
metadata:
  type: feedback
---

Ao implementar ML-1A (REQ-2026-09-12), `branch new` passou a resolver o start point via `origin/<padrão>`. Testes existentes que usam `makeE2ERepo` (sem remote) continuam funcionando no arm **negativo** (o gate de governança dispara antes da verificação de origin), mas o arm de **controle** (onde a governança passa e chega à resolução de start point) quebra com "nenhum remote 'origin' encontrado".

**Why:** o gate de governança (`BranchSlugMatchesRoadmap`) dispara antes de `resolveBranchStartPoint`. Só quando a governança passa é que o origin é verificado.

**How to apply:**
- Ao mudar `branch new` para exigir origin, adicionar `makeBareOriginAndPush(t, repoDir, homeDir)` em todos os testes existentes que chamam `branch new` (mesmo os que testam outra coisa).
- Para testes onde `branch new` leva a um checkout real: também fazer `git push origin main` depois do último commit local (antes do `branch new`), senão o checkout de `origin/main` não contém os arquivos recém-comitados.
- Testes que só precisam do arm negativo (governança bloqueia antes do origin check) não precisam de origem.

**Pattern medido:**
- `TestBranchStateE2E_AC2_DoneOnlyBlocksCreation`: arm negativo OK; arm de controle (--dry-run) quebrava.
- `TestBranchStateE2E_AC4_*`: `branch new` passa a governança, mas `roadmap move` falha porque o checkout de `origin/main` não tinha o roadmap — push antes do `branch new` resolve.
