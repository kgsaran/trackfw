---
status: wip
date: 2026-09-12
req: "docs/req/REQ-2026-09-12-branch-new-cria-a-partir-do-head-atual-e-nao-ha-comando-para-voltar-a-base-a-branch-nasce-empilhada-e-o-guard-nao-pode-bloquear-o-checkout-cru.md"
squad: ""
---

# Roadmap: branch new cria da base correta e o guard fecha o checkout cru

> Created: 2026-09-12 | Status: wip

## Contexto

REQ: docs/req/REQ-2026-09-12-branch-new-cria-a-partir-do-head-atual-e-nao-ha-comando-para-voltar-a-base-a-branch-nasce-empilhada-e-o-guard-nao-pode-bloquear-o-checkout-cru.md

`branch new` executa `git checkout -b <branch>` sem start point (`internal/commands/branch.go:141`).
A branch nasce empilhada sobre o HEAD atual em silêncio. PR #333 chegou com 20 commits alheios por
esse motivo.

## Critérios de aceite consolidados

- [ ] AC1: `branch new <type>/<slug>` cria a partir de `origin/main` por padrão, independente do HEAD
- [ ] AC2: `--from <ref>` cria a partir da ref dada; `--from HEAD` reproduz o comportamento atual
- [ ] AC3: faz `fetch origin` antes de resolver a base; se falhar, avisa em vez de bloquear
- [ ] AC4 (falsificação): na branch com commits próprios, `branch new` sem `--from` produz branch com zero commits além de `origin/main`; com `--from HEAD` os commits são preservados
- [ ] AC5 (BREAKING declarado): ir para CHANGELOG; decisão de versão — ver Decisões
- [ ] AC6: guard bloqueia `git checkout <branch>` e `git pull` crus — só após AC1 entregue
- [ ] AC7: `git checkout` liberado onde não há alternativa: worktree, `--detach`, `main`
- [ ] AC8: ~~paridade nos 3 CLIs~~ — OBSOLETO desde v8 (implementação única em Go)
- [ ] AC9: `--dry-run` informa qual base seria usada

## Legenda de status
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

---

## Wave 0 — Threat Model
> Dependências: nenhuma. Bloqueia toda implementação.

### ML-0A — Threat model para este roadmap
**Status:** ✅ Concluído — hades-tf 2026-10-18

**Arquivos lidos:** `internal/commands/branch.go`, `internal/guard/gitbranch.go`,
`internal/commands/branch_prune_forge.go`, `internal/commands/branch_prune.go`,
`internal/commands/ship.go` (incl. `buildPushArgs`), `internal/commands/push.go`,
`internal/generators/agentfiles.go`, `scripts/trackfw-git-branch-guard.sh`,
`docs/cli-parity.md` (seção `trackfw branch new`)

**Critérios de aceite:**
- [x] As quatro seções respondidas com evidência, não afirmação de uma linha
- [x] Nenhuma linha de implementação escrita neste ML

---

#### 1. Completude de superfícies

**Grep de enumeração (rodado, saída completa dos sítios relevantes):**

```
git grep -nE '"(checkout|switch|pull|worktree|branch)"' -- 'internal/' ':!*_test.go'

internal/commands/branch.go:141:  exec.Command("git", "checkout", "-b", branchName)
internal/commands/branch_prune.go:530: gitExec("branch", "--format=%(refname:short)")
internal/commands/branch_prune.go:559: gitExec("worktree", "list", "--porcelain")
internal/commands/branch_prune.go:581: gitExec("branch", "-d", name)
internal/commands/branch_prune.go:584: gitExec("branch", "-D", name)
internal/commands/ship.go:767:    gitExec("branch", "-r", "--no-merged", defaultRemoteRef)
internal/guard/gitbranch.go:479   case "checkout":
internal/guard/gitbranch.go:498   case "switch":
internal/guard/gitbranch.go:577   case "branch":
internal/guard/gitbranch.go:610   case "worktree":
```

Não existe nenhum outro `exec.Command("git", "checkout", "-b", ...)` nem `switch`, `-c` fora de
`branch.go:141` e dos cases do guard. A lista é fechada para o sítio de CRIAÇÃO.

**Superfície primária — sítio de criação:**

| Arquivo | Linha | Padrão atual | Observação |
|---|---|---|---|
| `internal/commands/branch.go` | 141 | `exec.Command("git", "checkout", "-b", branchName)` | ÚNICO sítio de criação de branch via `branch new` |
| `internal/commands/branch.go` | 196 | `[dry-run] would create branch %q (git checkout -b %s)` | dry-run message precisa mostrar a base (AC9) |

**Sítios secundários afetados (não criam branch, mas precisam de atualização):**

| Arquivo | Linha | Problema vivo |
|---|---|---|
| `internal/commands/push.go` | 145, 153 | instrui `git checkout -b feat/<slug>` — inconsistente com o guard que já bloqueia isso hoje |
| `internal/commands/ship.go` | 247, 255 | mesma instrução de `git checkout -b` |
| `internal/guard/gitbranch.go` | 55 (`reasonCheckoutPath`) | diz "`git checkout <branch>`/`git switch <branch>` seguem liberados" — torna-se falso sob AC6 |
| `internal/generators/agentfiles.go` | 1949 | `gitDenyPattern = ^git (commit\|push\|checkout -b)` — regex de deny gerado para integração; se AC6 adicionar formas, precisa de atualização |
| `scripts/trackfw-git-branch-guard.sh` | — | ainda presente (`ls scripts/`), instalado globalmente por `trackfw update harness`; qualquer nova regra no guard Go precisa de espelho aqui (ADR-2026-08-17) |
| `docs/cli-parity.md` | seção `trackfw branch new` | contrata o decision flow atual (step 5: `git checkout -b`), o output de `--dry-run`, e a ausência de `--from`; AC1/AC2/AC9 precisam de atualização desta seção |

**Lacunas do guard que criam o mesmo bug por caminhos alternativos (medido no binário):**

| Comando | Exit atual | Comportamento git | Nota |
|---|---|---|---|
| `git switch feat/foo` (apenas remota) | 0 PERMITIDO | DWIM: cria local com `@{u}` configurado | branch criada sem governança |
| `git checkout feat/foo` (apenas remota) | 0 PERMITIDO | DWIM: mesma lacuna | idem |
| `git checkout --track origin/x` | 0 PERMITIDO | cria local com `@{u}` | criação silenciosa |
| `git checkout -t origin/x` | 0 PERMITIDO | idem | idem |
| `git switch main` | 0 PERMITIDO | troca para branch existente | sem criação de branch |
| `git switch -` | 0 PERMITIDO | volta para branch anterior | sem criação de branch |
| `git checkout -` | 0 PERMITIDO | idem | sem criação de branch |
| `git worktree add ../foo` (sem -b) | 0 PERMITIDO | cria branch `foo` a partir do HEAD — **mesmo bug de empilhamento**, caminho diferente | ver ML-1C |

A forma `git worktree add <path>` sem `-b` é o mesmo causa raiz desta REQ por outro caminho: branch
nasce empilhada sobre o HEAD sem fetch nem start point. O guard bloqueia `worktree add -b` mas não
a forma sem flag (medido). Pelo critério da Regra Dura de Causa Raiz, entra neste roadmap como ML
próprio — ver ML-1C.

DWIM e `--track` criam a partir de `origin/<branch>`, não do HEAD local — causa diferente. Ficam
como resíduo declarado (R-2).

---

#### 2. Ameaça (quem esvazia a Wave 0 sem quebrar regra escrita)

O adversário é o implementador apressado, não um atacante externo.

**A-1 — Implementar `--from` mas não mudar o padrão.**
`branch new` aceita `--from origin/main` mas mantém HEAD quando a flag é omitida. AC1 diz "por
padrão" — o implementador pode ler como "quando o usuário não passa `--from`". O AC4 executado
em git real (não stub) é o que pega isso.

**A-2 — Hardcodar `"origin/main"` em vez de reutilizar `resolveDefaultBranch`.**
`resolveDefaultBranch` já existe em `branch_prune_forge.go` (forge API → `symbolic-ref
refs/remotes/origin/HEAD` → fallback `"main"`). Hardcodar quebra projetos com branch principal
`master`/`develop`/`trunk` silenciosamente: `checkout -b <branch> origin/main` falha com
`fatal: not a commit` (exit 128), sem mensagem de trackfw.

**A-3 — Colocar fetch e start-point dentro de `if branchGatedTypes[...]`.**
`chore/docs` pulam o bloco de governança. Se fetch e start-point ficarem nesse bloco,
`branch new chore/my-task` ainda nasce empilhada. AC1 e AC3 precisam cobrir todos os tipos.

**A-4 — Usar stub injetável para o AC4.**
`execGitCheckout` é campo injetável em `branchNewDeps`. Um teste unitário com stub satisfaz o
texto de AC4 sem rodar git. O teste passa, prova nada. AC4 precisa de repositório temporário real.

**A-5 — Pular `--no-track` e deixar como "escolha do implementador".**
`git checkout -b <branch> origin/main` configura `@{u} = origin/main` (medido). `buildPushArgs`
em `ship.go:843` chama `git rev-parse @{u}`: se upstream já está configurado, push roda SEM `-u`.
Resultado: o primeiro `trackfw push` não seta `origin/feat/slug` como upstream — o tracking
permanece `origin/main`. Após o merge e deleção de `origin/feat/slug`, `git branch -vv` mostra
`[origin/main: ...]` em vez de `[origin/main: gone]`. O Passo 0 do CLAUDE.md §1 usa
`[origin/X: gone]` como sinal primário — o sinal some. `--no-track` é **requisito**, não opção.

**A-6 — Bloquear `git checkout main` em AC6 sem alternativa.**
O guard é um matcher de strings sem I/O git. Bloquear `git checkout main` sem oferecer caminho
governado viola ADR-2026-08-17 e leva o usuário a desligar o guard. Se decisão D-1 = (a), AC6
se torna resíduo (ver Decisões D-2).

---

#### 3. Alvos de falsificação

**F-1: Base padrão (AC1) — direção negativa e positiva**
- FN: `branch new feat/foo` estando numa branch com commits alheios ainda cria da HEAD →
  o AC4 com git real deve falhar (`git rev-list --count origin/main..feat/foo` > 0)
- FP: `branch new feat/foo --from HEAD` cria da `origin/main` em vez do HEAD →
  o contra-braço do AC4 deve falhar (`git rev-list --count origin/main..feat/foo` = 0 indevido)

**F-2: Fetch e aviso (AC3)**
- FN: fetch falha silenciosamente, comando prossegue sem nenhum texto de aviso →
  teste com `gitExec` injetável mockando falha de fetch: saída deve conter o texto de aviso
- FP: fetch falha e `branch new` aborta com exit não-zero →
  mesma fixture: o comando deve criar a branch (com aviso), não abortar

**F-3: Chore/docs empilham (A-3)**
- FN: `branch new chore/my-task` estando numa branch com commits alheios cria branch empilhada →
  AC4-equivalente para tipos chore/docs (git real, `rev-list --count` > 0 indica falha)
- FP: não aplicável (o comportamento correto é criar da base)

**F-4a: Dirty tree conflitante → exit não-zero (BREAKING)**
- FN: `branch new feat/foo` com uncommitted change em F (onde F tem conteúdo diferente em
  `origin/main` e no HEAD atual) deve propagar o exit 1 do git — medido: `git checkout -b
  feat/test origin/main` nesse cenário → exit 1, `error: Your local changes to the following
  files would be overwritten by checkout: F. Aborting`
- Este é um comportamento BREAKING: hoje `branch new feat/foo` sem start point nunca falha por
  dirty tree (cria da HEAD, que é exatamente onde o uncommitted change está)
- O AC5 (BREAKING declarado) deve cobrir também este caso, não apenas o empilhamento

**F-4b: Repo sem remoto (decision D-3)**
- FN: `branch new feat/foo` num repositório sem `origin` propaga `fatal: not a commit` do git
  (exit 128) sem mensagem acionável de trackfw →
  o comando deve detectar ausência de `origin` antes de chamar git e emitir mensagem clara

**F-5: Upstream tracking após o fix (A-5 — `--no-track` obrigatório)**
- FN: após AC1, `git rev-parse --abbrev-ref feat/slug@{u}` não é vazio →
  indica que `--no-track` não foi usado; `trackfw push` não setará `origin/feat/slug` como
  upstream corretamente; Passo 0 do CLAUDE.md §1 fica cego ao merge/deleção da branch
- FP: não aplicável (`--no-track` não muda o comportamento funcional, apenas evita o side-effect)

**F-6: Mensagem do dry-run (AC9)**
- FN: `branch new --dry-run feat/foo` diz `would create branch "feat/foo" (git checkout -b
  feat/foo)` sem mencionar qual base usaria → violação do AC9
- O output esperado deve nomear a base: `[dry-run] would create branch "feat/foo" from origin/main`

**F-7: Shell guard como segundo sítio (A-6)**
- FN: `scripts/trackfw-git-branch-guard.sh` não é atualizado quando AC6 adiciona nova regra →
  usuários com o script global instalado ficam sem a nova proteção; o ADR-2026-08-17 documenta
  a exigência de paridade

**F-8: Mensagens de push.go/ship.go**
- FN: após AC1, `push.go:145` e `ship.go:247` continuam instruindo `git checkout -b feat/<slug>`
  — o usuário recebe instrução para um comando que o guard bloqueia; `grep -rn 'git checkout -b'
  internal/commands/{push,ship}.go` deve retornar vazio após Wave 1

---

#### 4. Resíduo declarado

**R-1: AC6/AC7 dependem de D-1 — e os equivalentes de pull ficam abertos**
Se D-1 = (a), o dano que AC6 pretendia prevenir desaparece (ver Decisão D-2). Se AC6 fosse
implementado mesmo assim, os seguintes comandos ficariam abertos por serem equivalentes funcionais
de `git pull` não cobertos pelo guard atual (todos exit 0):
`git fetch origin && git merge origin/main`, `git rebase origin/main`,
`git reset --soft origin/main`, `git pull --rebase`. Bloquear `pull` sem bloquear esses é
proteção parcial sem alternativa governada — viola ADR-2026-08-17.

**R-2: DWIM e formas `--track` — causa diferente, resíduo declarado**
`git checkout feat/foo` e `git switch feat/foo` (DWIM quando só existe no remote) criam local
tracking branch. `git checkout --track origin/x` e `-t` fazem o mesmo. A causa é diferente da
desta REQ: criam a partir de `origin/<branch>`, não empilham sobre HEAD. O guard não pode
distinguir DWIM de checkout read-only sem I/O git. Ficam como resíduo até REQ específica.

**R-3: Repo local-only sem `origin`**
`git checkout -b feat/test origin/main` em repo sem remote → exit 128 (medido). O caso
"repositório nunca conectado" fica coberto apenas pela mensagem acionável do ML-1A (F-4b).

**R-4: Dirty tree sem conflito com `origin/main`**
Uncommitted changes em arquivos rastreados cujo conteúdo é IGUAL em origin/main e em HEAD
(e arquivos não rastreados) são preservados na nova branch — comportamento git padrão (medido).
Não é bug; AC4 cobre apenas commits, o que é correto.

---

**Gates da wave:**
```bash
count=$(grep -c "^#### [1-4]\." "docs/roadmaps/wip/ROADMAP-2026-09-12-branch-new-cria-da-base-correta-e-o-guard-fecha-o-checkout-cru.md" 2>/dev/null || echo 0); [ "$count" -eq 4 ] || { echo "GATE FAIL: esperado 4 secoes numeradas no ML-0A, encontrado $count"; exit 1; }
go build -o bin/trackfw ./cmd/trackfw && bin/trackfw guard git-branch --command "git checkout -b test" </dev/null >/dev/null 2>&1; [ $? -eq 2 ] || { echo "GATE FAIL: guard nao bloqueia checkout -b (exit 2 esperado)"; exit 1; }
```

---

## Decisões que precisam do usuário

### D-1 — AC5: mudar o padrão é BREAKING. Qual versão e qual grau de mudança?

**Tabela de impacto por opção:**

| Cenário | Hoje (HEAD) | (a) origin/main sempre | (b) recusar se empilha | (c) só avisar |
|---|---|---|---|---|
| Na `main`, igual a `origin/main` | branch de HEAD=origin/main | idêntico | idêntico | idêntico |
| `main` local atrás de `origin/main` | branch nasce de main desatualizada | branch nasce do tip de origin/main ✓ | idem ✓ | branch ainda desatualizada, aviso |
| Feature branch A, commits acidentais | EMPILHA (bug do PR #333) | nasce de origin/main ✓ | recusa, sugere --from HEAD | ainda empilha, avisa |
| Feature branch A, empilhamento intencional | funciona | BREAK: precisa de `--from HEAD` explícito | mesma quebra, via recusa | funciona (sem proteção) |
| Dirty tree sem conflito (F-4a, arquivos iguais) | nunca falha | sem conflito, preserva arquivos ✓ | idem | idem |
| Dirty tree com conflito (F-4a, arquivo diferente entre HEAD e origin/main) | nunca falha | exit 1 do git BREAK | idem | idem |
| `fetch origin` falha (offline) | ignora (sem fetch) | avisa e cria de origin/main stale (AC3) | idem | idem |
| Repo sem `origin` | exit 128 do git | mensagem acionável de trackfw, exit não-zero | idem | idem |

**Nota sobre (b) com aviso:** aviso sem recusa é idêntico a (c) nos efeitos práticos. Com recusa,
o exit code muda — breaking da mesma ordem de (a), menos claro e mais surpreendente (funciona
em alguns casos e recusa em outros baseado em estado git).

**Nota sobre dirty-tree com conflito:** tanto (a) quanto (b) introduzem exit 1 em cenários que
hoje saem com exit 0 — o breaking não é só o empilhamento, é também esse caso.

**Recomendação: opção (a), versão 10.0.0.** O breaking é real mas cirúrgico: afeta quem depende de
empilhamento silencioso (que era bug) e quem tem dirty tree com conflito entre HEAD e origin/main
(raro, e o git já dá mensagem clara). A opção (c) não teria impedido o PR #333. A opção (b) tem o
mesmo custo de breaking sem a vantagem da clareza.

---

### D-2 — AC6/AC7: há uma contradição — bloquear `checkout main` e `pull` sem alternativa

**Estado atual (medido no binário v9.8.0):**

| Comando | Exit atual | Detalhe |
|---|---|---|
| `git checkout main` | 0 PERMITIDO | |
| `git pull origin main` | 0 PERMITIDO | |
| `git checkout feat/foo` | 0 PERMITIDO | DWIM se só-remota |
| `git checkout -b feat/bar` | 2 BLOQUEADO | ✓ |
| `git switch -c feat/baz` | 2 BLOQUEADO | ✓ |
| `git switch feat/foo` | 0 PERMITIDO | mesma lacuna DWIM |
| `git checkout --track origin/x` | 0 PERMITIDO | |
| `git checkout -t origin/x` | 0 PERMITIDO | |
| `git fetch origin` | 0 PERMITIDO | |
| `git checkout --detach HEAD` | 0 PERMITIDO | |
| `git checkout -- README.md` | 2 BLOQUEADO | descarte de caminho |
| `git checkout .` | 2 BLOQUEADO | descarte de caminho |
| `git worktree add ../foo` (sem -b) | 0 PERMITIDO | cria branch do HEAD (ver ML-1C) |

**A contradição:** o fluxo real deste projeto após cada merge é `git checkout main && git pull
origin main`. O escopo negativo proíbe criar comando novo. Se AC6 bloquear `git checkout main` e
`git pull`, não há caminho governado — ADR-2026-08-17: guard sem alternativa é desligado.

**O argumento que resolve, se D-1 = (a):** `branch new` faz o fetch e cria de `origin/main`
automaticamente. Um agente que pula `git checkout main && git pull` ainda obtém uma branch limpa.
O harm que AC6 pretendia prevenir desaparece.

**Opções:**
- **(i) Bloquear só checkout para branch-de-trabalho:** o guard precisaria de I/O git para distinguir
  "branch de trabalho" de "branch padrão" — nova latência e nova superfície de fail-open.
- **(ii) Separar AC6 em REQ futura:** AC6 é independente de AC1; pode ser entregue sozinho depois.
- **(iii) Mover AC6/AC7 para o resíduo se D-1 = (a):** o harm já está coberto.

**Recomendação: opção (iii) se D-1 = (a); opção (ii) se D-1 = (c).**

---

### D-3 — Base padrão em projetos com branch principal diferente de `main`

**O que já existe:** `resolveDefaultBranch(gitExec, ghExec)` em `branch_prune_forge.go` usa três
fontes: forge API → `git symbolic-ref --short refs/remotes/origin/HEAD` → fallback `"main"`.
É o mesmo padrão do `branch prune` — reutilizar.

**Para repo sem remote (medido):** `git checkout -b feat/test origin/main` → exit 128
`fatal: 'origin/main' is not a commit and a branch 'feat/test' cannot be created from it`.
O `branch new` deve detectar a ausência de `origin` antes de chamar git e emitir:
`trackfw branch new: nenhum remote 'origin' encontrado — use --from HEAD para criar a partir do HEAD atual, ou adicione um remote com 'git remote add origin <url>'`.

**Nota sobre `refs/remotes/origin/HEAD`:** um `git init` + `git remote add` sem fetch subsequente
não cria `refs/remotes/origin/HEAD`. A `resolveDefaultBranch` já contempla isso (cai no fallback).
O `branch new` pode reutilizar `resolveDefaultBranch` e depois verificar que a ref resolvida
existe como ref git antes de passar para o checkout — se não existir, emite mensagem acionável.

**Recomendação: reutilizar `resolveDefaultBranch`; verificar que a ref existe antes do checkout;
falhar explicitamente com mensagem acionável se `origin` não existir.**

---

## Wave 1 — Implementação
> Dependências: Wave 0 concluída; D-1, D-2, D-3 respondidas pelo usuário.

### ML-1A — `branch new` cria da base correta (AC1, AC2, AC3, AC4, AC9)
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/commands/branch.go`, testes

**Ações:**
1. Adicionar flag `--from <ref>` ao `newBranchNewCmd`
2. Reutilizar `resolveDefaultBranch` para obter a base padrão
3. Adicionar fetch com aviso (molde de `branch_prune.go:406`)
4. Verificar que a ref resolvida existe antes de chamar git (D-3)
5. Passar start point para `execGitCheckout` COM `--no-track` (obrigatório — F-5/A-5)
6. Cobrir chore/docs: também partem da base correta (A-3/F-3)
7. Atualizar mensagem de dry-run (AC9): mostrar qual base seria usada
8. Cobrir: repo sem `origin` emite mensagem acionável (F-4b)

**Critérios de aceite:**
- [ ] `git rev-list --count origin/main..feat/slug` = 0 após `branch new feat/slug` estando N commits à frente
- [ ] `git rev-list --count origin/main..feat/slug` = N após `branch new feat/slug --from HEAD` estando N commits à frente
- [ ] `branch new` com fetch mockado como falho: avisa e cria a branch (exit 0)
- [ ] `branch new chore/my-task` estando em branch com commits alheios: `rev-list --count` = 0
- [ ] `branch new feat/foo` sem remote: mensagem acionável de trackfw, exit não-zero
- [ ] `git rev-parse --abbrev-ref feat/slug@{u}` retorna vazio (--no-track funcionando)
- [ ] `--dry-run` mostra qual base seria usada, não apenas o nome da branch
- [ ] build e testes verdes

### ML-1B — Atualizar mensagens e contratos (F-7, F-8, cli-parity.md)
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/commands/push.go`, `internal/commands/ship.go`,
`internal/guard/gitbranch.go`, `scripts/trackfw-git-branch-guard.sh`, `docs/cli-parity.md`

**Ações:**
- `push.go:145,153` e `ship.go:247,255`: substituir `git checkout -b feat/<slug>` por `trackfw branch new feat/<slug>`
- `reasonCheckoutPath`: atualizar se AC6 for implementado (D-2)
- `docs/cli-parity.md` seção `trackfw branch new`: atualizar decision flow (step 5 muda), output de `--dry-run`, nova flag `--from`
- Shell guard: verificar paridade com qualquer mudança de AC6

**Critérios de aceite:**
- [ ] `grep -rn 'git checkout -b' internal/commands/{push,ship}.go` retorna vazio
- [ ] `docs/cli-parity.md` tem `--from <ref>` documentado
- [ ] build e testes verdes

### ML-1C — `git worktree add <path>` sem `-b` cria branch do HEAD (mesma causa)
**Status:** ⬜ Pendente
**Arquivos afetados:** `internal/guard/gitbranch.go`, `scripts/trackfw-git-branch-guard.sh`

**Base:** `git worktree add <path>` sem `-b` cria branch nomeada pelo último componente de
`<path>`, a partir do HEAD atual — mesmo bug de empilhamento, caminho diferente. Medido: exit 0
no guard hoje. Pela Regra Dura, entra neste roadmap.

**Ações:**
- Adicionar regra no guard: `git worktree add <path>` sem `-b`/`-B`/`--detach`/`--orphan` e sem
  `<commit-ish>` (ou onde `<commit-ish>` implica HEAD) → bloquear, sugerir `trackfw branch new`
- Espelhar no shell guard

**Critérios de aceite:**
- [ ] `git worktree add ../foo` (sem -b) → exit 2 do guard
- [ ] `git worktree add ../foo -b feat/bar` → exit 2 (já bloqueado hoje, confirmar regressão)
- [ ] `git worktree add ../foo origin/main` → exit 0 (detached HEAD, seguro)
- [ ] build e testes verdes

### ML-1D — CHANGELOG e bump de versão (AC5)
**Status:** ⬜ Pendente — aguarda D-1
**Arquivos afetados:** `CHANGELOG.md`, arquivos de versão

**Ações:**
- Entrada de breaking change para `branch new`: nova base padrão, dirty-tree-com-conflito agora exit 1
- Bump para 10.0.0 (se D-1 = opção a)

**Critérios de aceite:**
- [ ] `CHANGELOG.md` tem entrada de breaking change para `branch new`
- [ ] versão reflete o major bump

---

## Triagem pós-v8 — 2026-10-17 (REQ-2026-10-17)

**Vivo**: `branch.go:143` faz `git checkout -b` sem start point nem `fetch`. AC de paridade sumiu com a v8.

Evidência e tabela completa: `docs/governanca/2026-10-17-triagem-backlog-pre-v8.md`.

## Decisões do usuário — 2026-10-18

- **D-1: base padrão sempre a branch padrão do remoto (`origin/main`) → 10.0.0.** `branch new` faz
  `fetch` (avisa se falhar, molde do `branch prune`) e cria de `origin/<padrão>` resolvida por
  `resolveDefaultBranch` (o mesmo do `branch prune`); `--from <ref>` (inclusive `--from HEAD`) mantém
  o empilhamento intencional; `--no-track` obrigatório (senão o `@{u}` vira `origin/main`); repo sem
  `origin` → mensagem acionável. Entra no CHANGELOG como **BREAKING** e a próxima release é **major**.
- **D-2: AC6/AC7 saem desta REQ** como resíduo declarado — com o D-1 o dano que o bloqueio de
  `checkout <branch>`/`pull` evitaria desaparece, e bloquear `pull` sem caminho governado contraria o
  ADR-2026-08-17.
- **D-3:** branch padrão por `resolveDefaultBranch`, como no `branch prune`.
- **ML-1C fica** (mesma causa: `git worktree add <path>` sem `-b` cria branch do HEAD) — o guard já
  bloqueia `worktree add -b`; passa a bloquear a forma que cria branch implicitamente, deixando passar
  `--detach`, `<path> <branch-existente>` e `<path> <commit>`.
- **ML-1D:** CHANGELOG em `[Unreleased]` como BREAKING; o bump de versão vai no PR da release.
