# Bootstrap de repositório novo — achados do Wave 0 (2026-11-26)

> REQ-2026-11-26 | Hades | vcs.revision=59053568

## Achados críticos

### 1. `is-inside-work-tree` "false"/rc=0 em bare e `.git/` → D1 quatro estados

`git rev-parse --is-inside-work-tree` retorna "false" com rc=0 dentro de `.git/` e em repos
bare. Se D1 tratar rc=0/stdout=="false" como "não é repo", roda `git init` nesses diretórios e
cria repos aninhados: `.git/.git/` (dentro de .git dir) ou `.git/` dentro de bare (medido M-X9).
Adicionalmente, "dubious ownership" retorna rc=128 — `git init` a partir de subdir cria `.git/`
DENTRO do parent repo (medido M-D1owner via `GIT_TEST_ASSUME_DIFFERENT_OWNER=1`). D1 deve ter
quatro estados: rc=0/true→skip; rc=0/false→recusa; rc=128/LC_ALL=C stderr contém "not a git
repository"→init; rc=128/outro stderr (dubious ownership, permissão)→avisa rc=0.

### 2. `rev-list --all --count` (sem HEAD) é o único discriminador de "repo genuinamente vazio"

`git rev-list --all --count HEAD` → rc=128 tanto em fresh repo quanto em `switch --orphan` com
história. `git rev-parse --verify -q HEAD` → rc=1 em AMBOS. Único discriminador: `rev-list
--all --count` (sem HEAD) → "0" apenas em fresh. Predicate de D2 (commit): rc==1 AND count==0.

### 3. `git remote get-url --push` (sem `--all`) retorna apenas o primeiro pushurl

Com dois pushurls (PP1=vazio, PP2=tem main SHA diferente), `get-url --push` retorna PP1. D3
baseada nesse resultado permite push → PP1 recebe main, PP2 rejeita (stale info), rc=1 overall
(partial publish confirmado M-X10). D3 deve usar `get-url --push --all` e checar TODOS os
pushurls via `ls-remote`.

### 4. Anchor flip — "lacks main" abre via fresh init legítima

Com D3 "lacks main": fresh `trackfw init` + `git remote add origin <repo existente com develop>`
+ commit + push → D3 vê develop mas não main → permite → remote existente recebe main. Nenhuma
operação destrutiva necessária. "Remoto vazio" fecha o anchor flip em todos os casos. Decisão
pendente do KG.

### 5. D4 tem caminho próprio em `runShip` (ship.go:417)

`runShip` não chama `runPush`. Push de ship: `buildPushArgs` em ship.go:417. D4 implementado
apenas em `runPush` não bloqueia `tfw ship` de feature em remote vazio. ML-1B deve cobrir ambos.

### 6. D2 `branch new` deve ser adicionado antes de `runBranchNew:line 215`

Com `--from HEAD` em unborn repo: `resolveBranchStartPoint` vai para o case `fromRef == "HEAD"`
(line 293) e sucede (renomeia unborn para feat/x — o relato path). Sem `--from`, vai para line
330 (origin check) e retorna "no origin" hint — rc=1 certo, mensagem errada. D2 adicionado antes
da linha 215 dispara em ambos os caminhos. Gate para ML-1A deve verificar a mensagem de bootstrap.

### 7. `gh repo create --push` não interceptado pelo guard

Guard analisa subcomandos de `git`. `gh repo create --push` passa integralmente (medido M-C).
Todas as variantes (plain, --source=., bash -c) → rc=0. `git push` → rc=2. REQ linha 49 inclui.

### 8. Guard-allowed ref-movers colocam commits não revisados em main (M-mover)

`git reset --soft feat/x` (rc=0), `git merge feat/x` (rc=0), `git fetch . feat/x:main` (rc=0)
passam pelo guard. `git fetch . feat/x:main` medido: main count vai de 1 para 2 com SHA de feat/x.
Sob ambos os predicates de D3, `trackfw push` publicaria esses commits no bootstrap push. `git
merge` passa pelo guard enquanto `git commit` é bloqueado — assimetria importante para testes.
Questão 3 pendente: bound em D3 (`rev-list --count main == 1`) ou aceitar como residual.

## Arquivos de referência

- `docs/seguranca/2026-11-26-wave0-bootstrap-de-repositorio-novo.md` — documento completo
- `internal/commands/branch.go:291-335` — `resolveBranchStartPoint`, check order
- `internal/commands/ship.go:417` — push próprio de `runShip`
- `internal/commands/push.go:143,150` — bloco main/master e isShipBranch
- `internal/guard/gitbranch.go:849-860` — gap `switch --orphan`
