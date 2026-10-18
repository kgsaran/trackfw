---
name: branch-new-checkout-b-com-start-point-seta-upstream-no-track-obrigatorio-2026-10-18
description: git checkout -b <branch> origin/main seta @{u}=origin/main; buildPushArgs lê @{u}; --no-track é REQUISITO para o chain trackfw push funcionar corretamente
metadata:
  type: project
---

## Achado

`git checkout -b feat/test origin/main` configura automaticamente `feat/test@{u} = origin/main`
(medido com `git rev-parse --abbrev-ref feat/test@{u}`).

`buildPushArgs` em `internal/commands/ship.go:843` usa `git rev-parse @{u}` para decidir se
adiciona `-u`:

```go
func buildPushArgs(branch string, gitExec …) []string {
    _, err := gitExec("rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
    if err != nil {
        // Sem upstream — push com -u para configurar.
        return []string{"push", "-u", "origin", branch}
    }
    return []string{"push", "origin", branch}  // ← upstream já configurado, sem -u
}
```

**Consequência se `--no-track` não for usado no AC1 de REQ-2026-09-12:**

1. `branch new feat/slug` cria a branch com `@{u} = origin/main`
2. `trackfw push` encontra upstream já configurado → roda `git push origin feat/slug` SEM `-u`
3. Após o push, a tracking permanece `origin/main` (não `origin/feat/slug`)
4. Após o PR ser mergeado e `origin/feat/slug` ser deletada, `git branch -vv` exibe
   `[origin/main: ...]` em vez de `[origin/main: gone]`
5. O Passo 0 do CLAUDE.md §1 usa `[origin/X: gone]` como sinal primário para identificar
   branches integradas — o sinal some, Passo 0 fica cego para essa branch

**Fix:** usar `--no-track` no `git checkout -b <branch> <start-point>` ao implementar AC1:

```go
c := exec.Command("git", "checkout", "--no-track", "-b", branchName, startPoint)
```

**Status:** não corrigido (AC1 ainda não implementado). Declarado como requisito obrigatório
(não opção) no roadmap ROADMAP-2026-09-12 ML-1A.

**Why:** sem `--no-track`, o `trackfw push` ignora o passo `-u` que corretamente configuraria
`origin/feat/slug` como upstream. Isso não é apenas cosmético: o Passo 0 do CLAUDE.md depende
de `[origin/X: gone]` para a limpeza de branches, e a ausência do sinal correto causa trabalho
perdido (branches antigas não identificadas como integradas).

**How to apply:** sempre que um comando do trackfw fizer `git checkout -b <branch> <ref>`, usar
`--no-track` a menos que o caso de uso exija tracking explícito (e documentar isso).
