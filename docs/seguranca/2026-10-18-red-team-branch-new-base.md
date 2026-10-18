# Red-team Wave 1 — branch new com base correta (REQ-2026-09-12)

**Data:** 2026-10-18  
**Revisor:** hades-tf  
**Branch auditada:** `feat/branch-new-cria-da-base-correta`  
**Veredito:** BLOQUEIA  

---

## Escopo medido

```
git diff origin/main...HEAD -- internal/ docs/cli-parity.md CHANGELOG.md
```

Arquivos alterados: `internal/commands/branch.go`,
`internal/commands/branch_new_base_e2e_test.go`,
`internal/commands/branch_state_e2e_test.go`,
`internal/commands/branch_test.go`,
`internal/commands/branchlink_ml3a_test.go`,
`internal/commands/push.go`,
`internal/commands/ship.go`,
`internal/guard/gitbranch.go`,
`internal/guard/gitbranch_test.go`,
`docs/cli-parity.md`,
`CHANGELOG.md`.

Scripts: zero alterações — `scripts/trackfw-git-branch-guard.sh` é um thin wrapper que delega ao binário Go; qualquer nova regra do guard é propagada automaticamente.

`go test ./...` completo: verde (47s de e2e + todos os pacotes). `make lint`: verde.
`make quality`: falha em `check-wheel-filename.sh` por ausência do módulo Python `packaging` — pré-existente no ambiente, nenhum arquivo Python alterado neste PR.

---

## B-1 (BLOQUEANTE) — Injeção via `--from <flag>`: `--from -f` descarta trabalho não commitado

**Onde:** `internal/commands/branch.go`, função `resolveBranchStartPoint`.

**Código:**
```go
case deps.fromRef != "":
    // Explicit ref: use as start point directly.
    return deps.fromRef, nil
```

`fromRef` é passado sem qualquer validação para `defaultGitCheckout`, que o acrescenta diretamente como último argumento do `exec.Command("git", "checkout", "--no-track", "-b", branchName, startPoint)`.

**Reprodução medida:**

Setup: repositório com `origin` (bare), arquivo `file.txt` rastreado com conteúdo "original" confirmado, modificado para "UNCOMMITTED WORK" (sem commit).

```
$ trackfw branch new --from "-f" "chore/inject-test"
Switched to a new branch 'chore/inject-test'
Exit: 0
$ cat file.txt
original          ← trabalho perdido silenciosamente
```

Git executou: `git checkout --no-track -b chore/inject-test -f`.
O `-f` foi interpretado como `--force`, descartando as alterações não commitadas.
Sem aviso. Sem erro. Exit 0.

**Por que é bloqueante:** o trabalho é destruído de forma irreversível e silenciosa. O critério "deixar o repositório pela metade" se aplica: o usuário confia que `trackfw branch new` é seguro (o guard já bloqueia `git checkout -b` direto), mas esse caminho contorna o guard.

**Comparação com o guard:**
```
$ trackfw guard git-branch --command "git checkout -b feat/test -f"
→ deny (reasonCheckoutB)   ← guard funciona
$ trackfw branch new --from "-f" "chore/inject"
→ git checkout -f, exit 0  ← bypass confirmado
```

**Outros flags testados:**
- `--from --discard-changes`: git retorna 129 com usage error; arquivo preservado. Seguro (git rejeita).
- `--from -m` (merge mode): arquivo preservado com `M file.txt`; não destrutivo no caso simples, mas modifica o comportamento do checkout sem aviso. Ressalva menor incluída em R-2.
- `--from --force`: bloqueado pela govenance gate (sem roadmap correspondente para o slug testado). Não testado com `chore/` explicitamente — a causa raiz B-1 já cobre todos os valores com prefixo `-`.

**Fix necessário (uma linha):**
```go
case deps.fromRef != "":
    if strings.HasPrefix(deps.fromRef, "-") {
        return "", fmt.Errorf("branch new: --from %q: refs cannot start with '-' (use a branch or commit name, not a flag)", deps.fromRef)
    }
    return deps.fromRef, nil
```

`rev-parse --verify` adicional não substitui essa guarda: ele verificaria se a ref existe, mas `-f` não é uma ref — o problema é que git interpreta valores com `-` como flags, não como refs.

---

## Vetores limpos

### Base e remoto

| Cenário | Resultado | Observação |
|---|---|---|
| Remote `master` — non-dry-run | VERDE — branch criada de `origin/master` | fetch seta `origin/HEAD → master`; `resolveDefaultBranch` retorna `origin/master` |
| Remote `master` — `--dry-run` | FALHA com mensagem — `origin/main` not found | Ver R-1 abaixo (ressalva) |
| Repo sem origin | Exit não-zero, mensagem acionável | Contém `--from HEAD` e `origin`; sem raw "not a commit" |
| `--from HEAD` | Preserva commits da branch atual | F-1 contra-braço confirmado |
| `--from origin/main` | Branch criada de `origin/main` | Ref explícita passada diretamente |
| `--from <sha>` | Branch criada do SHA | SHA passado como start point |

### Fetch

| Cenário | Resultado |
|---|---|
| Fetch com URL inválida | Warning exibido, branch criada de ref local estale; exit 0 |
| Remote avança entre clone e branch new | Fetch atualiza, branch nasce do tip atual (AC3) |

### Árvo suja

| Cenário | Resultado |
|---|---|
| Alterações não-conflitantes (arquivo igual entre HEAD e origin/main) | Branch criada, arquivo preservado |
| Conflito (arquivo diferente entre HEAD e origin/main) | git exit 1, hint exibido, branch NÃO criada, HEAD inalterado, arquivo preservado — repositório limpo |

### Guard de worktree (ML-1C)

| Comando | Guard | git real | Resultado |
|---|---|---|---|
| `git worktree add ../foo` | BLOQUEIA (worktree-add-implicit) | cria branch `foo` | Correto |
| `git worktree add -f ../foo` | BLOQUEIA (1 positional, -f não é hasExplicit) | cria branch `foo` com force | Correto |
| `git worktree add ../foo main` | PASSA | checkout de branch existente `main` | Correto |
| `git worktree add ../foo origin/main` | PASSA | detached HEAD em origin/main | Correto |
| `git worktree add ../foo HEAD` | PASSA | detached HEAD | Correto |
| `git worktree add --detach ../foo` | PASSA (hasExplicit=true) | detached HEAD | Correto |
| `git worktree add -d ../foo` | PASSA (hasExplicit=true) | detached HEAD | Correto |
| `git worktree add -B x ../foo` | BLOQUEIA (worktree-add-b, regra pré-existente) | cria/reseta branch x | Correto |
| `git worktree add --track -b x ../foo origin/x` | BLOQUEIA (worktree-add-b) | cria branch x | Correto |
| `git -C repo worktree add ../foo` | BLOQUEIA | cria branch implícita | Correto (`-C` consumido por findSubcommand) |
| `(git worktree add ../foo)` | BLOQUEIA | cria branch implícita | Correto (parênteses stripped) |

### Push/ship mensagens

`grep 'git checkout -b' internal/commands/{push,ship}.go` retorna vazio. F-8 satisfeito.

### Testes novos — reconciliação

| Teste | Afirma |
|---|---|
| `TestBranchNewBase_DefaultCreatesFromRemote` | AC1/F-1: `branch new` sem `--from` produz branch com 0 commits além de `origin/main` |
| `TestBranchNewBase_FromHEAD_PreservesCommits` | F-1 contra-braço: `--from HEAD` preserva commits da branch atual |
| `TestBranchNewBase_ChoreDefaultCreatesFromRemote` | F-3/A-3: tipo `chore/` também cria de `origin/main`, não do HEAD |
| `TestBranchNewBase_NoTrackUpstream` | F-5/A-5: `@{u}` vazio após `branch new` sem `--from` — `--no-track` confirmado |
| `TestBranchNewBase_FetchUpdatesBase` | AC3: branch nasce do tip remoto atual após fetch, não do stale local |
| `TestBranchNewBase_FetchFailureWarnsAndCreates` | F-2/AC3: fetch falho → warning + branch criada; exit 0 |
| `TestBranchNewBase_NoOriginActionable` | F-4b: repo sem origin → exit não-zero + mensagem acionável, sem "not a commit" |
| `TestBranchNewBase_DryRunShowsBase` | AC9/F-6: `--dry-run` nomeia a base ref ("origin/...") |

Todos os 8 testes passam. Todos reprovam sem a correção que afirmam (verificado via estrutura: sem start point, `rev-list --count` retornaria > 0; sem `--no-track`, `rev-parse @{u}` retornaria valor; etc.).

**Ressalva de reconciliação R-5** (ver abaixo): `TestBranchNewBase_NoOriginActionable` não discrimina especificamente o caminho `remote get-url` — o teste passaria mesmo se o early-exit fosse removido e a mensagem viesse do `rev-parse` downstream. O comportamento observável está correto; o caminho exato não está pinado.

---

## Ressalvas

### R-1 — `--dry-run` diverge do run real para remotes sem `origin/HEAD` prévia (master/develop)

**Medido:** remote bare com branch padrão `master`, sem `origin/HEAD` local configurada.

- `--dry-run`: pula o fetch → `origin/HEAD` não está configurada → `resolveDefaultBranch` cai no fallback `"main"` → `origin/main` não existe → falha com mensagem.
- Run real: fetch seta `refs/remotes/origin/HEAD → origin/master` → `resolveDefaultBranch` retorna `origin/master` → branch criada com sucesso.

A mensagem de falha do `--dry-run` ("verifique se o remote foi configurado corretamente e tente 'git fetch origin'") é tecnicamente correta mas confusa: o fetch já resolveria o problema, e é exatamente o que o run real faz. O usuário que rodar `--dry-run` antes do run real receberá um falso negativo.

Não é bloqueante (o run real funciona; nenhuma destruição de trabalho).

### R-2 — Guard FN: `git worktree add --lock --reason <text> <path>` cria branch implícita

**Medido:**
```
git worktree add --lock --reason my-reason /tmp/why
→ Preparing worktree (new branch 'why')
```

O guard conta `my-reason` (valor de `--reason`) + `/tmp/why` (caminho) = 2 positionals, passa. git cria branch `why` a partir do HEAD.

A raiz é que o contador de positionals não modela flag arguments (`--reason <value>` consome um token). Flags similares afetadas: qualquer flag `--foo <value>` onde `<value>` não começa com `-`.

Não é bloqueante por critério (FN, não FP de forma legítima).

### R-3 — Guard FN: `git worktree add --orphan <path>` cria branch implícita, guard passa

**Medido (git 2.54.0 / Apple Git-157):**
```
git worktree add --orphan /tmp/orphan-test
→ Preparing worktree (new branch 'orphan-test')
```

O guard trata `--orphan` como `hasExplicit = true` (igual a `--detach`). Mas neste git, `--orphan` sem `-b` usa o path component como nome da branch — mesmo comportamento de criação implícita que esta REQ bloqueia. Comportamento pode variar por versão de git.

Não é bloqueante por critério (FN).

### R-4 — `cli-parity.md` referencia "pre-v9.9" mas a versão breaking é v10.0.0

**Localização:** `docs/cli-parity.md`, linha 1983.
```
`--from <ref>` | Override the start point. `--from HEAD` reproduces the pre-v9.9 behaviour
```

O CHANGELOG e as decisões do usuário dizem 10.0.0. A string "v9.9" no contrato de paridade está incorreta.

Não é bloqueante (comportamento funcional não afetado), mas deve ser corrigido antes do merge para não contaminar o contrato.

### R-5 — `TestBranchNewBase_NoOriginActionable` reconciliação fraca (ver seção de testes)

O teste afirma "a repo without 'origin' gets an actionable message and exit non-zero, never a raw git exit 128". O teste passaria mesmo se o bloco `remote get-url` fosse removido (o `rev-parse` downstream também produz exit não-zero e mensagem contendo `--from HEAD` e `origin`). O comportamento observável está correto; o caminho específico não está pinado.

---

## Regressão

| Comando | origin/main | HEAD | Delta esperado? |
|---|---|---|---|
| `git worktree add ../foo` | PASSA | BLOQUEIA | SIM (ML-1C) |
| `git worktree add ../foo main` | PASSA | PASSA | — |
| `git worktree add --detach ../foo` | PASSA | PASSA | — |
| `git worktree add -b x ../foo` | BLOQUEIA | BLOQUEIA | — |
| `git checkout -b feat/test` | BLOQUEIA | BLOQUEIA | — |

Sem regressão fora do delta esperado.

---

## Veredito e ação necessária

**BLOQUEIA.**

Causa: B-1 — `trackfw branch new --from "-f"` executa `git checkout --no-track -b <branch> -f`, descartando trabalho não commitado irreversivelmente. A destruição de trabalho é o critério de bloqueio mais direto ("deixar o repositório pela metade").

**Fix obrigatório antes do merge:**

Em `internal/commands/branch.go`, função `resolveBranchStartPoint`, adicionar antes de `return deps.fromRef, nil`:

```go
if strings.HasPrefix(deps.fromRef, "-") {
    return "", fmt.Errorf("branch new: --from %q: refs cannot start with '-'", deps.fromRef)
}
```

Adicionar teste unitário que affirma essa verificação (Regra Dura de Reconciliação):
reconciliation sentence: "rejects a --from value that starts with '-', exits non-zero without calling git".

**Itens a corrigir antes do merge também:**
- R-4: corrigir "pre-v9.9" para "pre-v10" em `docs/cli-parity.md` linha 1983.

**Ressalvas que podem ficar para após o merge (não bloqueiam):**
- R-1: divergência `--dry-run` / run real para remote `master` — documentar como resíduo ou abrir issue.
- R-2: guard FN `--lock --reason <text> <path>` — abrir issue/ML.
- R-3: guard FN `--orphan <path>` — documentar como resíduo.
- R-5: reconciliação fraca do `TestBranchNewBase_NoOriginActionable` — não bloqueia, mas o teste deveria usar um stub injetável para pinnar o caminho específico.
