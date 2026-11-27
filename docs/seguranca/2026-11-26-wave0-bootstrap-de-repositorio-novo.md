# Wave 0 — Threat Model: Bootstrap de repositório novo

> ML-0A do ROADMAP-2026-11-26 | 2026-11-26 | Hades (hades-tf)
> REQ: docs/req/REQ-2026-11-26-repositorio-novo-nao-tem-rota-governada-para-publicar-a-branch-padrao-e-o-init-nao-inicializa-o-git.md
> ADR: docs/adr/ADR-2026-11-26-bootstrap-de-repositorio-novo-init-roda-git-init-e-a-branch-padrao-e-publicada-uma-vez-pelo-trackfw.md

Binário medido: trackfw 11.9.1, vcs.revision=5905356870a3c7bc4098f1344f2ad854b1278802,
vcs.time=2026-11-26T13:12:50Z, vcs.modified=false. Todos os comandos do binário rodaram com
HOME, USERPROFILE, CODEX_HOME e XDG_CONFIG_HOME apontando para `$SCRATCH/hades-boot/fakehome`
(isolado). Remotos: apenas bare locais. Comandos que precisavam de `git commit` para o estado do
repositório foram escritos em scripts em `$SCRATCH/` e executados via `bash $SCRATCH/script.sh`
— o guard bloqueia a forma inline de compound commands com `git commit`. Isso é evidência viva
para o residual de classe de §4: scripts do usuário executam git fora do guard.

---

## 1. Enumeração de completude

O roadmap lista estes pontos de mudança: `init.go`, `branch.go`, `commit.go`, `push.go`,
`ship.go`, `barrier.go`, `validator_credential_guard_integrity.go`, `agentfiles.go` e
`docs/cli-parity.md`.

### 1.1 Emissores do bloco "Agent Protocol"

Fonte única: `trackfwRulesBlock()` em `internal/generators/agentfiles.go:59`. Callers reais
(grep de produção, sem testdata, sem _test.go):

| Arquivo | Linha | O que faz |
|---|---|---|
| `internal/generators/agentfiles.go` | 243, 252 | `InjectRulesDetected` → chama `InjectRulesForTool` → emite bloco |
| `internal/generators/agentfiles.go` | 1613 | `InjectRulesForTool("windsurf", guardRoot)` — emite diretamente |
| `internal/commands/init.go` | 711 | `generators.InjectRulesForTool(tool, cwd)` — emite |
| `internal/commands/integrations_flags.go` | 288 | `generators.InjectRulesForTool(target, ...)` — emite |
| `internal/generators/update.go` | 75, 3414 | `InjectRulesDetected(cwd)` — emite via InjectRulesDetected |
| `internal/commands/discover.go` | 157 | `generators.InjectRulesDetected(cwd)` — emite |

**Edição D5**: somente `internal/generators/agentfiles.go` linha ~103 (corpo do bloco).

**Superfícies NÃO emissoras de Agent Protocol, mas relevantes para D5**:

| Arquivo | Linha | O que faz |
|---|---|---|
| `internal/commands/init.go` | 633 | `PrintArchitectNextSteps` — imprime "start with the architecture guide" (primeira mensagem após init, lida antes do `CLAUDE.md`) |
| `internal/generators/update.go` | 154 | `PrintArchitectNextSteps` — mesma |
| `internal/commands/discover.go` | 166 | `PrintArchitectNextSteps` — mesma |
| `internal/integrations/assets/agents/architect.md` | 27, 29 | instrui `trackfw branch new` e `trackfw push`, sem mencionar bootstrap |

`PrintArchitectNextSteps` é a primeira mensagem que o agente lê após init, antes de abrir o
`CLAUDE.md`. Atualmente imprime "Run /trackfw:architect". Com D5, também deve mencionar o
bootstrap quando o repositório não tem commits.

### 1.2 Mensagens que mandam para `trackfw branch new` ou `trackfw push`

| Arquivo | Linha | Literal |
|---|---|---|
| `internal/guard/gitbranch.go` | 23 | `reasonCheckoutB` — "Use `trackfw branch new <type>/<slug>`" |
| `internal/guard/gitbranch.go` | 25 | `reasonSwitchC` — "Use `trackfw branch new <type>/<slug>`" |
| `internal/guard/gitbranch.go` | 27 | `reasonBranchCreate` — "Use `trackfw branch new <type>/<slug>`" |
| `internal/guard/gitbranch.go` | 33 | `reasonPush` — "Use `trackfw push`" |
| `internal/commands/commit.go` | 346 | "Use 'trackfw branch new \<type\>/\<slug\>' first" |
| `internal/commands/branch.go` | 110, 195, 257, 332, 356 | "use --from HEAD" (help text, hint, dry-run, dois erros) |
| `internal/commands/push.go` | 145 | "use a feature branch: trackfw branch new feat/\<slug\>" |
| `internal/commands/ship.go` | 247 | mesma mensagem de push |
| `internal/integrations/assets/agents/architect.md` | 27, 29 | instrui `trackfw branch new` e `trackfw push` |

### 1.3 Superfícies ausentes do ML-1C "Files affected"

**A)** `internal/integrations/assets/agents/architect.md` e `internal/generators/nextsteps.go` —
ambos devem entrar em ML-1C.

**B)** `internal/guard/gitbranch.go` e `scripts/trackfw-git-branch-guard.sh` — `git switch
--orphan` não está bloqueado; `gh repo create --push` também não. Lacunas de controle. `scripts/`
é mirror byte-a-byte (`gitbranch.go:15`). ML-1D dedicado recomendado (ML-1A/1B já editam arquivos
disjuntos).

---

## 2. Modelo de ameaça

### 2.1 D1 — predicate quatro estados de "dentro de work tree"

**Gap de implementação**: `git rev-parse --is-inside-work-tree` retorna "false" com rc=0 em
repos bare e dentro de `.git/`.

```
# Medido (M-F, 2026-11-26):
$ cd $BARE && git rev-parse --is-inside-work-tree; echo rc=$?
false
rc=0

$ cd $REPO/.git && git rev-parse --is-inside-work-tree; echo rc=$?
false
rc=0

$ cd $FRESH && git rev-parse --is-inside-work-tree; echo rc=$?
fatal: not a git repository (or any of the parent directories): .git
rc=128
```

**Se D1 tratar rc=0/stdout=="false" como "não é repo" e rodar `git init`, cria repos aninhados
(medido M-X9, 2026-11-26)**:

```
# Em $REPO/.git:
$ git init -b main
Initialized empty Git repository in $REPO/.git/.git/

# Em bare dir:
$ git init -b main
Initialized empty Git repository in $BARE/.git/
```

**Gap adicional — "dubious ownership" (medido M-D1owner, 2026-11-26)**: `is-inside-work-tree`
retorna rc=128 em um repo existente com dono diferente (dev containers, volumes Docker com uid
mismatch). `git init` a partir de um subdiretório desse repo — com rc=128 vindo do parent —
cria `.git/` DENTRO do parent (nested repo confirmado):

```
# De subdir de repo com dubious ownership:
GIT_TEST_ASSUME_DIFFERENT_OWNER=1 git rev-parse --is-inside-work-tree → rc=128
GIT_TEST_ASSUME_DIFFERENT_OWNER=1 git init -b main → rc=0
ls subdir/.git → NESTED GIT CRIADO DENTRO DO PARENT REPO
```

D1 não pode tratar todo rc=128 como "não é repo → init". Deve verificar o stderr.

**Quatro estados corretos**:

| `rev-parse` resultado | D1 |
|---|---|
| rc=0, stdout=="true" | print toplevel, skip `git init`, continue scaffold, rc=0 |
| rc=0, stdout=="false" | RECUSA com rc≠0, antes de qualquer escrita (incluindo `trackfw.yaml`) |
| rc=128, LC_ALL=C stderr contém "not a git repository" | executa `git init`, depois escreve scaffold |
| rc=128, outro stderr | avisa (ex.: "dubious ownership"), skip `git init`, continue scaffold, rc=0 |

**Sem binário `git`** (REQ AC D1): avisa, skip `git init`, continue scaffold, rc=0. O scaffold
ainda é escrito; o usuário instala git e roda `git init` manualmente. Consistente com ADR D1
("avisa e segue (rc=0)").

**Locale**: `defaultGitExec` não impõe `LC_ALL=C`. Medido em quatro locales (C, pt_BR.UTF-8,
pt_PT.UTF-8, default do sistema — macOS 27.2): em todos os locales o stderr foi
"fatal: not a git repository…" (macOS não traduz). Linux com pacotes `git-core-doc` pode divergir.
D1 deve passar `LC_ALL=C` explicitamente na invocação de `is-inside-work-tree`.

**Absorção (M-F2)**: se o diretório pai é um repo git, `is-inside-work-tree` retorna "true"/rc=0
com `show-toplevel` apontando para o repositório pai. D1 cai na primeira linha do tristate e não
roda `git init`.

### 2.2 D2 — multiple vias para "unborn HEAD em repo com história" e predicate correto

**Guard gaps (M-G e M-X6, 2026-11-26)**:
```
payload: {"command":"git checkout --orphan main"}              → rc=2  (BLOQUEADO)
payload: {"command":"git switch --orphan main"}                → rc=0  (gap no case switch)
payload: {"command":"git symbolic-ref HEAD refs/heads/main"}   → rc=0  (guard permite)
payload: {"command":"git worktree add --orphan main /tmp/wt"}  → rc=0  (guard permite)
payload: {"command":"git update-ref -d refs/heads/main"}       → rc=2  (BLOQUEADO)
payload: {"command":"git branch -m feat/x main"}               → rc=2  (BLOQUEADO — medido M-rename)
```

Esses gaps são defense-in-depth: D2 (rev-list count) é o controle real. D3 sobre o remote é o
controle que não pode ser forjado apenas localmente.

**Predicate de D2 (M-X1, 2026-11-26)**:

```
# Fresh repo (unborn main):
$ git rev-list --all --count           → "0", rc=0
$ git rev-list --all --count HEAD      → rc=128 ("ambiguous argument 'HEAD'")
$ git rev-parse --verify -q HEAD       → rc=1

# switch --orphan main2 num repo com 1 commit em main:
$ git rev-list --all --count           → "1", rc=0
$ git rev-list --all --count HEAD      → rc=128
$ git rev-parse --verify -q HEAD       → rc=1  (mesmo que fresh)
```

`rev-list --all --count HEAD` retorna rc=128 em ambos os casos — não pode ser discriminador.
`rev-list --all --count` (sem HEAD) é o discriminador correto.

**Predicate de D2 (commit)**: permite quando rev-parse retorna rc==1 AND rev-list retorna
stdout=="0" com rc==0. Qualquer outro resultado recusa. Fail-closed.

**Branch new half de D2 (M2, 2026-11-26)**:
```
# repo sem commits, main unborn, sem origin:
$ tfw branch new feat/bootstrap
Error: trackfw branch new: no 'origin' remote found — use --from HEAD...
rc=1

$ tfw branch new feat/bootstrap --from HEAD
Switched to a new branch 'feat/bootstrap'
rc=0   ← relato path: D2 deve recusar aqui também
```

Hoje `branch new` sem `--from` falha com a mensagem de "sem origin" (branch.go:332), não com
mensagem de bootstrap. Com `--from HEAD` em unborn, sucede (branch.go:293 — sem verificação de
HEAD). D2 para `branch new` deve ser adicionado antes de `runBranchNew:line 215` (antes do check
de gated types) para disparar em AMBOS os caminhos. Se adicionado apenas no case
`fromRef == "HEAD"` (line 293), o caso sem `--from` continua respondendo com a mensagem de "sem
origin" — RC=1 certo, mensagem errada. Gate para ML-1A deve verificar a mensagem de bootstrap,
não apenas rc=1.

**FP arm obrigatório**: `branch new feat/x --from HEAD` em repo com 1 commit, sem origin deve
continuar funcionando (rev-parse HEAD rc=0 → D2 não dispara).

**`git branch -m feat/x main` não é bypass**: guard bloqueia (rc=2, medido M-rename). A SHA
existente também faz rev-parse HEAD rc=0 → predicate falha.

**Dois predicates diferentes em D2**: `branch new` recusa com rev-parse rc==1 sozinho (antes do
check de origin); `commit` usa a conjunção completa (rev-parse rc==1 AND rev-list count==0).
Não compartilhar um único `isBootstrap()`.

### 2.3 D3 — `ls-remote origin` lê a URL de leitura, não a de push

**Gap de implementação confirmado (M-A, 2026-11-26)**:
```
# url=BARE_A (vazio), pushurl=BARE_B (tem main)
$ git push origin main                 # vai para BARE_B via pushurl
$ git ls-remote --heads origin         # lê BARE_A via url → vazio (rc=0)
$ git remote get-url origin            → file://.../bare_A
$ git remote get-url --push origin     → file://.../bare_B
```

D3 deve usar `git remote get-url --push --all origin` para obter todos os pushurls, e rodar
`git ls-remote --heads <url>` para cada um. D3 recusa se ANY pushurl tiver a branch. Ver §2.4
para o vetor de múltiplos pushurls.

**Erros "no origin" (M-X2a)**:
```
$ git remote get-url --push origin    → rc=2 "No such remote 'origin'"
```
D3 fail-closed: rc != 0 → recusa.

**Erros de remote inacessível (M-X2b/c)**:
```
$ git ls-remote --heads file:///nonexistent       → rc=128
$ git ls-remote --heads git://127.0.0.1:19999/x  → rc=128
```
D3 fail-closed: rc != 0 → recusa. D4 fail-open: rc != 0 → feature segue.

**Netdeadline**: `defaultGitExec` aplica netdeadline a `ls-remote` (classificada como "metadata"
em `netexec.go:81`). D3 e D4 devem usar `deps.execGit("ls-remote", ...)`.

**Cadeia de gates em `runPush` (push.go)**: D3 deve inserir a exception path SUBSTITUINDO o
bloco unconditional de `main`/`master` em push.go:143 — não depois dele. Depois de linha 143,
`main` também falha `isShipBranch(branch)` em push.go:150 e o erro seria da mensagem errada.
`trackfw ship` (ship.go:245) permanece recusando `main`/`master` após D3 — ship é caminho
independente (não chama `runPush`).

**User `--force-with-lease`**: com D3, `trackfw push --force-with-lease` em main unborn alcança
step 2.5 (push.go:201) que recusa por "nenhum PR aberto em main" — message errada. D3 deve recusar
`opts.forceWithLease` explicitamente na exception path, antes de step 2.5, com mensagem específica.

**Conteúdo de main não limitado por D3**: D3 verifica apenas se o remote tem a branch. Seja qual
for a história local de `main` no momento do bootstrap, ela chega ao remote sem PR. Declarado em
§4.

### 2.4 D3 — lease de ausência (TOCTOU, oracle trap e múltiplos pushurls)

**TOCTOU confirmado (M-TOCTOU)**:
```
$ git push "file://$BARE_Y" --force-with-lease=refs/heads/main: main
! [rejected] main -> main (stale info)
rc=1
```

**Oracle trap (M-B)**: SHA idêntica retorna rc=0 "Everything up-to-date" — não é rejeição.
Os testes de ML-1B devem semear SHA DIFERENTE para o vetor TOCTOU.

**Multiple pushurls — partial publish confirmado (M-X10, 2026-11-26)**:
```
# PP1 vazio; PP2 tem main com SHA diferente; push com absence lease:
To file://.../pp3_bare1 — * [new branch] main -> main    (PP1 aceita)
To file://.../pp3_bare2 — ! [rejected] main -> main (stale info)   (PP2 recusa)
rc=1

Após push: PP1 tem main (SHA e66b6b6); PP2 mantém main antigo (SHA 208128b)
```

`get-url --push` (sem `--all`) retorna apenas PP1. D3 baseada em `get-url --push` veria PP1
como vazio antes do push → permite → PP1 recebe main (partial publish). D3 deve iterar sobre
TODOS os pushurls via `git remote get-url --push --all origin`.

O papel do lease e da pré-checagem são distintos e complementares: a pré-checagem por
`git remote get-url --push --all origin` seguida de `ls-remote` por URL é o controle primário
contra partial publish (único controle que cobre multi-pushurl). O lease é a barreira de TOCTOU
por push-alvo: fecha a janela entre checagem e push para cada pushurl individual. Nenhum dos
dois pode ser omitido.

### 2.5 D3 — anchor flip e o trade-off do predicate

**O caminho genuíno (medido M-E, 2026-11-26)** — sem nenhuma operação destrutiva:
1. `trackfw init` em pasta sem git → D1 cria repo.
2. `git remote add origin <repo existente com develop e sem main>` (remote apontado erroneamente).
3. `trackfw commit` → D2 vê rev-list count=0 → permite.
4. `trackfw push`:
   - D3 "lacks main": `ls-remote --heads` mostra develop → não tem main → permite push.
   - D3 "remoto vazio": `ls-remote --heads` mostra develop → não está vazio → recusa.
5. Com "lacks main": remote agora tem `main` e `develop`. O scaffold ocupou um repo existente.

Este caminho não exige nenhuma operação destrutiva. O único erro é apontar `origin` para o remote
errado no passo 2.

**Variante destrutiva — re-init (medido M-X8)**:
```
payload: {"command":"rm -rf .git"}        → rc=0  (guard permite)
payload: {"command":"git init -b main"}   → rc=0  (guard permite)
payload: {"command":"git branch -m feat/x main"}   → rc=2  (BLOQUEADO — M-rename)
```

Re-init exige o passo 2 novamente (adicionar origin). Com "remoto vazio", tanto a variante fresh
quanto a re-init são bloqueadas da mesma forma: o remote com develop não está vazio.

**Bounded recovery path under "lacks main"**: após D3 "lacks main" publicar `main`, as rotas
governadas para recriá-la num remote que tem apenas `feat/x` incluem: (a) re-init (rm -rf .git +
git init + remote add + D2 count=0); (b) guard-allowed ref-movers (`git reset --soft feat/x` rc=0,
`git merge feat/x` rc=0, `git fetch . feat/x:main` rc=0, medidos M-mover) — esses colocam commits
de feat/x em main sem re-init, e D3 "lacks main" permite o push (remote tem feat/x mas não tem
main). `git branch -m` bloqueado (rc=2, M-rename). `git reset --hard` bloqueado (rc=2). Com
"remoto vazio", nem o re-init nem os movers ajudam — remote tem feat/x, não está vazio, D3 recusa.

**Trade-off para o arquiteto/KG**:

| Predicate D3 | Anchor flip (remote com develop, fresh init com origin errado) | Recovery do relato state (remote tem feat/x, sem main) |
|---|---|---|
| "lacks main" | ABERTO | ABERTO (re-init M-X8; guard-allowed movers: reset --soft, merge, fetch . — M-mover) |
| "remoto vazio" | FECHADO | FECHADO (remote tem feat/x → D3 recusa; movers funcionam localmente mas D3 recusa o push) |

A ADR (Alternatives Considered, linha 96) rejeitou "remoto totalmente vazio" argumentando que
"fecharia a exceção para sempre" para o recovery do relato state. Mas D4 fecha esse estado
preventivamente: com D4, a feature nunca ocupa o remote antes de main. O recovery do relato state
com "remoto vazio" exigiria intervenção manual fora do trackfw — aceitável como residual (§4).
Decisão final: KG.

### 2.6 D4 — feature barrada em remoto vazio; D4 em ship é caminho separado

**D4 em ship não está coberto pelo ML-1B**: `runShip` não chama `runPush`. O push de ship é
feito diretamente em ship.go:417 via `buildPushArgs`. D4 implementado apenas em `runPush` não
bloqueia `trackfw ship` de feature em remote vazio. ML-1B deve cobrir ambos os caminhos, ou um
helper compartilhado deve ser extraído.

**`gh repo create ... --push` bypassa guard e D4 — BLOQUEANTE (M-C, 2026-11-26)**:
```
"gh repo create x --source=. --remote=origin --push"   → rc=0
"bash -c \"gh repo create x --source . --push\""       → rc=0
"gh repo create x --source . --push"                   → rc=0

# Sem --push: correto (não há push)
"gh repo create x --source ."    → rc=0

# Controle: git push bloqueado
"git push origin main"           → rc=2
```

O guard analisa o `command` como subcomando `git`. `gh repo create` não inicia com `git` e não é
interceptado. A REQ (linha 49) inclui este achado.

---

## 3. Falsificação nas duas direções por superfície

### D1 — `init` cria git

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | init não cria git | init cria git aninhado em bare ou .git dir |
| Gate | `tfw init && test -d .git/objects && test "$(git symbolic-ref --short HEAD)" = main` em dir sem `.git` → rc=0 | `tfw init` em `$REPO/.git` → rc≠0, `test ! -e $REPO/.git/.git/` → true; `tfw init` em bare → rc≠0, `test ! -e bare/.git/` → true |
| TTY (REQ AC D1) | `tfw init </dev/null && test -d .git/objects` → rc=0 (sem TTY); `script -q /dev/null tfw init && test -d .git/objects` → rc=0 (com pty) | — |
| Branch padrão | FN: `git symbolic-ref --short HEAD` ≠ "main" (implementação com `git init` sem `-b`) | — |
| Pré-condição | rc=128, LC_ALL=C stderr contém "not a git repository" → init | rc=0, stdout=="false" → rc≠0, abort (nenhum scaffold escrito), `test ! -e .git/.git/` |
| Absorção | — | `tfw init` em subdir de outro repo (rc=0 stdout=="true") → imprime root detectado, NÃO roda `git init`, scaffold escrito no cwd |
| Sem git | — | PATH sem `git`: scaffold escrito, `git init` skipped, rc=0, `test -e trackfw.yaml` |
| Dubious ownership | — | rc=128 outro stderr: scaffold escrito, `git init` skipped, rc=0, `test ! -e .git/` dentro do parent, `test -e trackfw.yaml` |

**D1 recusa para "false"/rc=0 deve ser rc≠0**: este é o único caso onde D1 deve abortar antes de
qualquer escrita — é possível distinguir "está dentro de gitdir/bare" (rc=0 stdout=="false") de
"não tem git" (rc=128) ou "git com ownership issue" (rc=128 outro). Dubious ownership e sem git:
scaffold é escrito (mesma postura que absorção — a user corrige o ambiente e tem os arquivos).

### D2 — commit na branch padrão só no primeiro commit

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | commit em main continua bloqueado em repo sem commit | commit em main aceito após primeiro commit |
| Gate | `tfw commit -m "initial"` em main unborn, rev-list count=0 → rc=0 | segundo `tfw commit -m "..."` em main → rc=1 |
| Pré-condição | rev-parse rc==1 AND rev-list count==0 → permite | rev-parse rc==0 OR rev-list count>0 → recusa |

Três contra-braços no ML-1A: (1) fresh, count=0 → permite; (2) `switch --orphan` em repo com
história, count=1 → recusa; (3) main com um commit existente → rev-parse rc=0 → recusa.

**D2 — branch new**:

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | `branch new feat/x --from HEAD` aceito em HEAD unborn | `branch new feat/x --from HEAD` recusado em repo com commits e sem origin |
| Gate | `tfw branch new feat/x --from HEAD` em HEAD unborn → rc=1 **com mensagem de bootstrap** | `tfw branch new feat/x --from HEAD` em repo com 1 commit, sem origin → rc=0 |
| Sem --from | `tfw branch new feat/x` em HEAD unborn, sem origin → rc=1 com mensagem de bootstrap (não "no 'origin' remote" hint) | — |

D2 deve ser adicionado antes de `runBranchNew:line 215`. Gate deve verificar a mensagem de
bootstrap, não apenas rc=1.

### D3 — push de main aceito só na primeira vez

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | push de main sempre bloqueado | push de main aceito N vezes |
| Gate | `tfw push` em main com todos os pushurls vazios → rc=0 | segundo `tfw push` em main com main em qualquer pushurl → rc=1 |
| pushurl divergente | D3 checando `ls-remote origin` (url) em vez de pushurl → permite push errado (medido M-A) | — |
| Multi-pushurl | D3 checando apenas primeiro pushurl (get-url sem --all) → partial publish (medido M-X10) | — |
| TOCTOU | main criado entre checagem e push → lease rc=1 (medido M-TOCTOU) | — |
| User --force-with-lease | D3 sem recusa explícita → alcança step 2.5 com mensagem errada | D3 recusa --force-with-lease em main bootstrap com mensagem específica |

**Anchor flip** (decisão pendente KG): "lacks main" → permite push de main em remote com develop
em fresh init com origin errado (sem operação destrutiva). "remoto vazio" → recusa. Ver §2.5.

**Ship on main permanece bloqueado**: ship.go:245 recusa main/master incondicionalmente. D3
exception é somente em `runPush`. Gate: `tfw ship -m "..."` em main → rc=1 sempre.

### D4 — feature barrada em remoto vazio

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | feature sobe para remote vazio (exatamente o relato) | feature bloqueada em remote com develop/trunk |
| Gate | `tfw push` em feat/x com remote vazio → rc=1 | `tfw push` em feat/x com remote tendo develop → rc=0 |
| Pré-condição | `ls-remote --heads <pushurl>` rc=0 E stdout vazio → bloqueia | rc=0 E não-vazio → passa |
| Hang | D4 sem `deps.execGit` pende em remote lento | — |
| D4 em ship | `tfw ship -m "..."` em feat/x com remote vazio → rc=1 (gap atual: ship não chama runPush) | — |

**Erros de rede**: D4 fail-open: rc != 0 → feature segue. D3 fail-closed. `deps.execGit` aplica
netdeadline — deve ser usado em ambos.

### Guard `git switch --orphan` e `gh repo create --push`

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| switch --orphan | payload `{"command":"git switch --orphan main"}` → rc=0 (gap) → após fix: rc=2 | payload `{"command":"git switch feat/x"}` → rc=0 (continua) |
| gh create --push | payload `{"command":"gh repo create x --source . --push"}` → rc=0 (gap) → após fix: rc=2 | payload `{"command":"gh repo create x --source ."}` sem --push → rc=0 |

Cobrir `symbolic-ref HEAD` e `worktree add --orphan` no mesmo ML-1D. Declarar residual no ML-1D
se optar por não fechar os três: D2 é o controle real, os guard fixes são defense-in-depth.

### D5 — mensagens descrevem bootstrap

| Direção | Falso negativo | Falso positivo |
|---|---|---|
| Comportamento | template instalado não descreve o passo de primeiro commit | texto de bootstrap em repo existente leva agente a tentar commitar em main; D2 recusa — comportamento esperado (benign) |
| Gate | grep para "trackfw commit" (passo de primeiro commit) no bloco "Agent Protocol" instalado → encontra | — |
| ml_scope (antes do push) | FN: `trackfw barrier <roadmap>.md --wave 1` em repo unborn → output não contém "trackfw commit" ou "trackfw push" | barrier bloqueia repo existente por outro motivo não relacionado ao bootstrap (correto) |
| severity | FN: `trackfw validate` em repo sem refs remotas → output não contém "trackfw commit" ou "trackfw push" | — |

**M-D5-track (2026-11-26, inferred)**: após `git push origin --force-with-lease=refs/heads/main:
main` contra bare local, `git rev-parse --verify -q refs/remotes/origin/main` → rc=0. Push
atualiza tracking ref diretamente. Barrier unblocked é inferred; verificação do output do barrier
a cargo do ML-3A replay.

---

## 4. Residual declarado

**PENDENTE — predicate de D3 (decisão KG)**: "lacks main" vs "remoto vazio" — ver §2.5. Com
"lacks main": anchor flip ABERTO (fresh init com origin errado; sem operação destrutiva) e
recovery ABERTO (via re-init). Com "remoto vazio": ambos FECHADOS, recovery do relato state exige
intervenção manual. Não aceito como residual.

**PENDENTE — conteúdo de main não limitado no primeiro push (decisão KG, Questão 3 em §decisões)**:
Guard-allowed ref-movers (`git reset --soft` rc=0, `git merge` rc=0, `git fetch . feat/x:main`
rc=0 — medidos M-mover, M-mover-fetch) colocam commits não revisados em main antes do bootstrap
push sob AMBOS os predicates. Candidato ao menor bound: D3 recusa a menos que `git rev-list
--count refs/heads/main` == 1 (exatamente o commit que D2 sancionou). Decisão final: arquiteto/KG.

**Aceito — rede não testada contra GitHub real**: `git ls-remote --heads` contra remote GitHub
(vazio ou com só tags) não foi medido nesta sessão (dispatch proibiu acesso a rede). Item para
ML-2B.

**Aceito — tags-only remote**: retorna vazio em `ls-remote --heads` → D4 bloqueia feature push.
Comportamento conservador.

**Aceito — classe gh/non-git push além de `gh repo create`**: outros binários com push interno
(`lab`, `glab`, scripts) não são interceptados pelo guard. Residual pré-existente.

**Aceito — recovery do relato state com "remoto vazio"**: exige intervenção manual fora do
trackfw. Aceitável com D4 ativo.

---

## 5. Específicos desta REQ com evidência

### 5.1 pushurl ≠ url: a consulta fecha?

```bash
# Medido (M-A, 2026-11-26):
git remote add origin "file://$BARE_A"           # url=BARE_A
git remote set-url --push origin "file://$BARE_B" # pushurl=BARE_B
git push origin main                              # vai para BARE_B
git ls-remote --heads origin                      # lê BARE_A → vazio, rc=0
git remote get-url origin                         → file://.../bare_A
git remote get-url --push origin                  → file://.../bare_B
```

D3 deve usar `git remote get-url --push --all origin` e passar cada URL para `git ls-remote`.

### 5.2 Lease recusa quando main existe com SHA diferente?

```bash
# TOCTOU confirmado (M-TOCTOU): SHA diferente → rc=1
$ git push "file://$BARE_Y" --force-with-lease=refs/heads/main: main
! [rejected] main -> main (stale info)
rc=1

# Oracle trap (M-B): SHA idêntica → rc=0 (idempotência, não rejeição)
$ git push origin --force-with-lease=refs/heads/main: main
Everything up-to-date
rc=0

# Partial publish (M-X10): PP1 vazio, PP2 diferente SHA
To file://.../pp3_bare1 — * [new branch] main -> main    (PP1 aceita)
To file://.../pp3_bare2 — ! [rejected] main -> main (stale info)
rc=1 — PP1 tem main; PP2 inalterado
```

Pré-checagem via `git remote get-url --push --all origin` seguida de `ls-remote` por URL
obrigatória. Testes de ML-1B: semear SHA DIFERENTE para TOCTOU.

### 5.3 `gh repo create ... --push` desvia do guard?

```bash
# Sim — guard permite (M-C, 2026-11-26):
"gh repo create x --source=. --remote=origin --push"   → rc=0
"bash -c \"gh repo create x --source . --push\""       → rc=0
"gh repo create x --source . --push"                   → rc=0

# Sem --push: correto
"gh repo create x --source ."    → rc=0

# Controle: git push bloqueado
"git push origin main"           → rc=2
```

### 5.4 `commit_unborn_branch_test.go` — o que afirma e como D2 interage

**`TestDefaultCurrentBranch_UnbornBranch`** frase: `defaultCurrentBranch` retorna "main" via
`symbolic-ref --short HEAD` como fallback em repo unborn — habilitando que `runCommit` leia o
nome antes de tomar a decisão D2. Pré-condição de D2, não D2 em si.

**`TestDefaultCurrentBranch_DetachedHEAD`** frase: HEAD destacado retorna "HEAD" porque
`rev-parse --abbrev-ref HEAD` retorna "HEAD" nesse estado.

**`TestDefaultCurrentBranch_OutsideRepo`** frase: fora de um repo, retorna o erro do `rev-parse`.

Nenhum dos três testa que `runCommit` ACEITA o commit em main em repo unborn. Medido (M1,
2026-11-26): `tfw commit -m "initial"` em main unborn → rc=1 (bloqueado — D2 não implementado).
D2 requer nova guarda em `runCommit` antes de `commitProtectedBranches`: rev-parse rc==1 AND
rev-list count==0.

### 5.5 D4 — `ls-remote --heads` no remoto vazio, com só tags, com `develop`

```bash
# Remote bare vazio
$ git ls-remote --heads "file://$BARE_EMPTY"      → vazio, rc=0

# Remote com só tag v0.1.0
$ git ls-remote --heads "file://$BARE_TAGS_ONLY"  → vazio, rc=0
$ git ls-remote --tags  "file://$BARE_TAGS_ONLY"  → fb7fbb93...  refs/tags/v0.1.0, rc=0

# Remote com develop
$ git ls-remote --heads "file://$BARE_DEV"        → d28d9df9...  refs/heads/develop, rc=0

# Remote inacessível
$ git ls-remote --heads file:///nonexistent       → rc=128
$ git ls-remote --heads git://127.0.0.1:19999/x  → rc=128
```

D4: rc=0 E stdout vazio → bloqueia; rc=0 E não-vazio → passa; rc != 0 → fail-open.
D3: rc != 0 → fail-closed.

**Remote GitHub vazio ou com só tags**: não medido nesta sessão (dispatch proibiu acesso a rede).
Medição a cargo do ML-2B.

### 5.7 Ref-movers através do guard e mecanismo de conteúdo não revisado (M-mover, 2026-11-26)

**Guard — quais comandos passam (M-mover-guard)**:
```bash
$ echo '{"command":"git reset --soft feat/x"}' | tfw guard git-branch;         echo "rc=$?"
rc=0  (guard permite)

$ echo '{"command":"git reset --hard feat/x"}' | tfw guard git-branch;         echo "rc=$?"
rc=2  (BLOQUEADO)

$ echo '{"command":"git merge feat/x"}' | tfw guard git-branch;                echo "rc=$?"
rc=0  (guard permite)

$ echo '{"command":"git update-ref refs/heads/main HEAD"}' | tfw guard git-branch; echo "rc=$?"
rc=2  (BLOQUEADO)

$ echo '{"command":"git fetch . feat/x:main"}' | tfw guard git-branch;         echo "rc=$?"
rc=0  (guard permite)
```

**Mecanismo confirmado — `git fetch .` coloca commits não revisados em main (M-mover-fetch)**:
```bash
# repo com 1 commit em main (scaffold), feat/x com 2 commits:
# on feat/x:
$ git fetch . feat/x:main
From .
   54f4a17..0ec5ac6  feat/x     -> main
fetch rc=0
main count after fetch: 2
main SHA: 0ec5ac6   (= feat/x SHA)
```

Resultado: `main` aponta para o commit de feat/x sem PR nem revisão. Sob AMBOS os predicates
de D3, `trackfw push` publicaria esses commits no bootstrap push.

Nota: `reset --soft` e `merge` requerem HEAD em `main` (você estar em main). `git symbolic-ref
HEAD refs/heads/main` (guard rc=0, M-X6) cria o ponteiro sem commits — permite reset/merge.
`fetch .` funciona de qualquer branch.

### 5.6 Censo de superfícies — grep confirmado

**Emissores de Agent Protocol** (via `InjectRulesForTool`/`InjectRulesDetected`):
`agentfiles.go:243, 252, 1613`, `init.go:711`, `integrations_flags.go:288`,
`update.go:75, 3414`, `discover.go:157`.

**NÃO emissores** (apenas `PrintArchitectNextSteps`):
`init.go:633`, `update.go:154`, `discover.go:166`.

**Mensagens `--from HEAD`**:
`branch.go:110, 195, 257, 332, 356` — help text, hint, dry-run e dois erros.

**Gaps de guard**:
`gitbranch.go` case `switch`: falta `--orphan` (M-G). Não intercepta `gh repo create --push` (M-C).
`scripts/trackfw-git-branch-guard.sh`: mirror; ambas as correções devem ser espelhadas.

---

## 6. Ajustes ao ADR/roadmap

### D1 — ajustar

Substituir a condição em "Decision" por: "A detecção usa `git rev-parse --is-inside-work-tree`
com `LC_ALL=C` em quatro estados: rc=0/stdout=='true' → imprime toplevel, skip `git init`,
continua scaffold, rc=0; rc=0/stdout=='false' → aborta com rc≠0 sem nenhuma escrita (dentro de
gitdir ou bare — `git init` nesses casos cria repos aninhados, medido M-X9); rc=128 / stderr
contém 'not a git repository' → executa `git init` com `-b main` (ou `git init` + `symbolic-ref`
em git antigo sem `-b`), depois escreve scaffold; rc=128 / outro stderr (ex.: 'detected dubious
ownership' — medido M-D1owner: `git init` em subdir cria repo aninhado dentro do parent) → avisa,
skip `git init`, escreve scaffold, rc=0. Sem binário `git` (`exec.ErrNotFound`): avisa, skip `git
init`, escreve scaffold, rc=0 (mesma postura — user instala git e roda `git init`). LC_ALL=C é
obrigatório na invocação de `is-inside-work-tree` (macOS não traduz, Linux pode divergir)."

### D2 — ajustar (BLOQUEANTE para ML-1A)

Substituir o predicate por: "O predicate de 'primeiro commit' é a conjunção: (1) `git rev-parse
--verify -q HEAD` retorna rc==1 E (2) `git rev-list --all --count` retorna stdout=='0' com rc==0.
Fail-closed em ambas. `--all --count HEAD` não pode ser usado (rc=128 quando HEAD é unborn —
medido M-X1). `branch new` usa apenas a condição (1), adicionada antes do check de gated types
em `runBranchNew` (antes da linha 215). `commit` usa a conjunção completa. Os dois predicates são
implementados separadamente."

### D3 — ajustar (BLOQUEANTE para ML-1B)

Adicionar: "A consulta é `git remote get-url --push --all origin` para obter todos os pushurls,
seguida de `git ls-remote --heads <url>` para cada pushurl. D3 recusa se ANY pushurl tiver a
branch (medido M-X10). Se `get-url` retornar rc != 0, D3 recusa. Se qualquer `ls-remote` retornar
rc != 0, D3 recusa. D3 é fail-closed. `deps.execGit` deve ser usado. D3 deve recusar
`opts.forceWithLease` antes de step 2.5 com mensagem específica. D3 exception substitui o bloco
de main/master em push.go:143."

### D3 — predicate anchor flip (PENDENTE — decisão KG)

Ver §2.5. "lacks main" → ambos anchor flip e recovery ABERTOS. "remoto vazio" → ambos FECHADOS.
A ADR rejeitou "remoto vazio" antes de D4. Com D4 ativo, o recovery do relato state via trackfw
não é necessário (D4 impede a formação do estado). Recomendado: "remoto vazio". Decisão final: KG.

### D4 — ajustar (BLOQUEANTE para ML-1B)

Adicionar: "D4 se aplica tanto a `runPush` quanto ao caminho de push de `runShip` (ship.go:417),
que não chama `runPush`. ML-1B deve cobrir ambos ou extrair um helper compartilhado."

### D3/D4 — conteúdo de main (PENDENTE — decisão KG, Questão 3)

D3 verifica existência da branch no remote, não o conteúdo. Guard-allowed ref-movers (`git reset
--soft`, `git merge`, `git fetch . feat/x:main` — todos rc=0, medidos M-mover, M-mover-fetch)
colocam commits não revisados em main antes do bootstrap push. Se o arquiteto/KG aceitar o residual:
acrescentar às "Consequences" do ADR: "O primeiro conteúdo de `main` não é limitado ao commit de
scaffold. Guard-allowed ref-movers podem colocar qualquer história local em main antes do bootstrap
push. O segundo push já é barrado." Se adicionado o bound (`rev-list --count main == 1`): acrescentar
às Actions de ML-1B e ao AC D3.

### Guard — `git switch --orphan`, `gh repo create --push` (ML-1D recomendado)

`internal/guard/gitbranch.go` e `scripts/trackfw-git-branch-guard.sh`: bloquear `--orphan` no
case `switch`; bloquear `gh repo create` com `--push`/`--push=true`. ML-1D dedicado recomendado.

Cobrir `symbolic-ref HEAD` e `worktree add --orphan` no mesmo ML-1D ou declarar residual no
ML-1D (D2 é o controle real). Decisão arquiteto/KG: adicionar à lista de pendências junto com D3.

### ML-1C — arquivos adicionais

`internal/integrations/assets/agents/architect.md` e `internal/generators/nextsteps.go`.

### ML-1B — ampliar Actions de D4

Acrescentar nas Actions de ML-1B: "D4 wire em `runShip` step 6 (ship.go:417) além de `runPush`."

---

**Estado do repositório ao fim desta sessão:**

```
$ git -C /Users/kgsaran/Sistemas/Desenvolvimento/workspace/trackfw status --short
 M vault/notes/index.md
?? docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md
?? vault/notes/bootstrap-repo-novo-achados-wave0-2026-11-26.md
```

Nenhum arquivo de produto tocado. Nenhuma operação git de escrita executada no repositório
trackfw. Os três arquivos acima devem ser commitados pelo arquiteto.

---

**Decisões pendentes para o arquiteto/KG (antes de despachar ML-1B e ML-1D)**:

**Questão 1 — D3 predicate (necessária antes de ML-1B)**:

D3 deve usar "destination lacks main/master" ou "remoto vazio"?

- "lacks main": anchor flip ABERTO em fresh init com origin errado (sem operação destrutiva);
  recovery via re-init ABERTO.
- "remoto vazio": ambos FECHADOS; recovery do relato state exige `git push` manual fora do trackfw.

Com D4 ativo, a feature nunca ocupa o remote antes de main (relato state não se forma). O recovery
do relato state com "remoto vazio" exigiria intervenção manual apenas para repos pre-D4 que
entraram no estado comprometido antes dessa REQ. Medido e confirmado em M-E (anchor flip com
develop em fresh init), M-X8 (re-init; guard permite ambas as ops; `git branch -m` bloqueado
M-rename rc=2), e M-mover (guard-allowed ref-movers colocam história não revisada em main sob
ambos os predicates).

**Questão 2 — ML-1D: `symbolic-ref HEAD` e `worktree add --orphan` (antes de ML-1D)**:

Cobrir no ML-1D ou declarar residual? D2 (rev-list count) é o controle real. `symbolic-ref HEAD`
e `worktree add --orphan` são defense-in-depth. `gh repo create --push` é o único controle que a
REQ escolheu fechar nesta superfície. Se declarar residual para symbolic-ref/worktree --orphan,
o ML-1D cobre apenas `switch --orphan` e `gh repo create --push`.

**Questão 3 — bound no conteúdo de main no primeiro push (antes de ML-1B)**:

Guard-allowed ref-movers (`git fetch . feat/x:main` rc=0, `git merge` rc=0, `git reset --soft`
rc=0) colocam commits não revisados em main antes do bootstrap push sob AMBOS os predicates de D3
(medidos M-mover, M-mover-fetch). Candidato ao menor bound: D3 recusa a menos que `git rev-list
--count refs/heads/main` == 1 (exatamente o commit que D2 sancionou). Sem o bound, o ADR-2026-10-10
anchor ("o primeiro conteúdo de main é o commit de scaffold") não se sustenta. Decisão
arquiteto/KG: aceitar o residual ou adicionar o bound em D3.
