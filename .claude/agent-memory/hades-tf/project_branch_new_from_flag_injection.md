---
name: project_branch_new_from_flag_injection
description: branch new --from "-f" passes value directly to git checkout, discarding uncommitted work; not fixed in Wave 1 (blocked); fix is to reject fromRef starting with "-"
metadata:
  type: project
---

## Achado

`trackfw branch new --from "-f"` executa `git checkout --no-track -b <branch> -f`.
O `-f` é interpretado pelo git como `--force`, descartando alterações não commitadas em arquivos rastreados silenciosamente. Exit 0. Sem aviso.

**Reprodução (medido 2026-10-18, REQ-2026-09-12 Wave 1):**
```
$ echo "UNCOMMITTED WORK" > file.txt
$ trackfw branch new --from "-f" "chore/inject"
Switched to a new branch 'chore/inject'
Exit: 0
$ cat file.txt
original    ← trabalho perdido
```

**Causa:** `resolveBranchStartPoint` em `branch.go` não valida `fromRef` quando ele não é `"HEAD"`:
```go
case deps.fromRef != "":
    return deps.fromRef, nil  // ← sem validação
```

**Status:** BLOQUEANTE B-1 no parecer `docs/seguranca/2026-10-18-red-team-branch-new-base.md`.
O PR foi bloqueado.

**Fix (uma linha):**
```go
if strings.HasPrefix(deps.fromRef, "-") {
    return "", fmt.Errorf("branch new: --from %q: refs cannot start with '-'", deps.fromRef)
}
```

**Why:** O guard bloqueia `git checkout -b feat/test -f` diretamente, mas `trackfw branch new --from -f` contorna o guard. Qualquer flag de git que altere comportamento do checkout pode ser injetada.

**How to apply:** Toda vez que um comando do trackfw aceitar uma string de ref como argumento CLI passada diretamente para git, validar que não começa com `-`. Isso é necessário mesmo quando `exec.Command` é usado (sem shell) porque o git interpreta argumentos com prefixo `-` como flags, não como refs.
