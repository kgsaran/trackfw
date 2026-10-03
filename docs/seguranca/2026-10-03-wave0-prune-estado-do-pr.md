---
status: wave0
date: 2026-10-03
reviewer: hades-tf
roadmap: docs/roadmaps/wip/ROADMAP-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md
req: docs/req/REQ-2026-10-03-branch-prune-classifica-branches-ja-mergeadas-como-pending-work-porque-nunca-consulta-o-estado-do-pr.md
---

# Wave 0 — Threat model: sinal de PR no `branch prune`

> Date: 2026-10-03 | Reviewer: hades-tf | AC1 da REQ-2026-10-03

ADR: `docs/adr/ADR-2026-10-03-branch-prune-usa-o-estado-do-pr-no-forge-como-sinal-de-integracao-com-o-conteudo-como-fallback-declarado.md`

---

## Seção 1 — Completude da enumeração

### 1.1 Pontos de decisão de integração no código atual

```bash
grep -rn "evaluateBranchIntegration\|branchPruneDecision\|detectPendingSquashMerges" \
  internal/ --include="*.go" | grep -v "_test.go"
```

Saída relevante (produção, excluindo comentários e strings de geração):

| arquivo | símbolo | papel |
|---|---|---|
| `internal/commands/branch_prune.go:93` | `evaluateBranchIntegration` | definição — decisão única |
| `internal/commands/branch_prune.go:368` | `evaluateBranchIntegration` | chamador: `runBranchPrune` |
| `internal/commands/ship.go:801` | `evaluateBranchIntegration` | chamador: `detectPendingSquashMerges` |
| `internal/commands/ship.go:786` | `detectPendingSquashMerges` | definição — aviso no push/ship |
| `internal/commands/push.go:235` | `detectPendingSquashMerges` | chamador: `runPush` |
| `internal/commands/ship.go:389` | `detectPendingSquashMerges` | chamador: `runShip` |

Grep adicional para `--no-merged`, `is-ancestor`, chamadas a `gh`:

```bash
grep -rn "\-\-no-merged\|is-ancestor\|exec.Command.*gh\|\"pr\", \"list\"" \
  internal/ --include="*.go" | grep -v "_test.go"
```

Resultados relevantes:

- `ship.go:787`: `git branch -r --no-merged origin/main` — entrada de `detectPendingSquashMerges`.
- `ship.go:197`: `gh pr list --head <branch> --state open --json number` — `defaultCheckPROpen`, usada por `runShip` para decidir se abre PR. **Não é ponto de decisão de integração de branch.**
- `internal/forge/adapter.go`: não chama `gh pr list`; chama apenas `gh pr create` (abertura de PR).
- `internal/generators/claudemd.go:169`: instrução gerada no `CLAUDE.md` de consumidores — não é código de decisão.

**Enumeração fechada**: a decisão de integração vive integralmente em `evaluateBranchIntegration` (`branch_prune.go:93`). `detectPendingSquashMerges` é o único outro consumidor e encaminha para o mesmo ponto. Não há terceiro sítio.

### 1.2 O que o ADR nomeia e o que falta

O ADR nomeia corretamente D1–D4. Quatro pontos não especificados que pertencem à implementação:

- **`baseRefName`** não aparece como campo obrigatório em D1 (ver Cenário 9).
- **Seleção do repositório alvo** (`--repo`): o ADR diz "repositório de origem é o próprio origin" mas não especifica o mecanismo de garantia (ver Cenário 5).
- **Predicate "sem upstream"**: o ADR descreve o comportamento desejado (step 4) mas não especifica como detectar "nunca empurrada" vs "upstream gone" (ver Cenário 7).
- **D4 — quais novos vereditos disparam o aviso**: o ADR não lista quais das novas constantes de `branchPruneDecision` devem substituir `branchPruneDecisionPendingWork` no filtro de `detectPendingSquashMerges`, nem trata o problema do prefixo `origin/` (ver Cenário 10).

---

## Seção 2 — Threat model

Adversário: o implementador apressado e o arquiteto otimista — quem lê o ADR, acha que cobre tudo, e entrega código que apaga trabalho não integrado sem violar nenhuma regra escrita.

Para cada cenário: comando de medição, saída, e veredito.

---

### Cenário 1 — PR de fork com o mesmo nome de head

**Mecanismo**: colaborador abre PR de `fork/feat/foo` com head branch `feat/foo` no fork. Dono tem
branch local `feat/foo` com trabalho não integrado. Se o PR do fork aparecer como MERGED, o filtro
deve excluí-lo.

**Medição**:

```bash
gh pr list --state all --limit 500 \
  --json number,state,headRefName,isCrossRepository \
  | python3 -c "
import json,sys
d=json.load(sys.stdin)
cross=[p for p in d if p['isCrossRepository']]
print('Cross-repo PRs:', len(cross))
for p in cross[:3]:
    print(' #%d %s %s' % (p['number'],p['state'],p['headRefName']))
"
# Cross-repo PRs: 1
#  #509 MERGED chore/ac2-nao-depende-do-acervo-real
# EXIT=0
```

PR #509: `isCrossRepository: true`. O filtro `isCrossRepository: false` exclui este PR corretamente.

**Importante**: o filtro de fork não é o controle que impede um delete indevido. Mesmo que
`isCrossRepository: false` falhe, o `is-ancestor` e o `baseRefName` ainda precisariam passar para
que o delete ocorra. Se o headRefOid do PR do fork estiver no mesmo grafo de commits (fork do mesmo
projeto), a `is-ancestor` pode passar — mas o conteúdo estaria em main de qualquer forma (o fork
PR foi mergeado). O filtro de fork serve para satisfazer AC3, não para segurança de delete.

**Veredito**: **coberto pelo ADR** (D1). O filtro `isCrossRepository` satisfaz AC3. O controle real
que previne delete de trabalho não integrado é `is-ancestor` combinado com `baseRefName`.

---

### Cenário 2 — Nome de branch reaproveitado depois de PR antigo mergeado

**Mecanismo**: developer usa `feat/foo` para PR #100 (MERGED, headRefOid = SHA_A). Depois apaga,
recria `feat/foo` com novo trabalho (tip = SHA_B, não relacionado a SHA_A). Nenhum PR aberto ainda.

**Medição — `headRefOid` é congelado no momento do merge**:

```bash
# PR 503: fix/criterio-de-adr-por-prefixo
gh pr view 503 --json headRefOid --jq .headRefOid
# c61300cacdf9bf6b91cad8b7864b33cfbd03ecfe  (momento do merge, 2026-10-02)

git rev-parse fix/criterio-de-adr-por-prefixo
# 99477f4492b77bcb746000b4633c2cc255e203dd  (1 commit posterior, push em 2026-10-02 13:13)

# É o tip (99477f4a) ancestral do headRefOid (c61300ca)?
git merge-base --is-ancestor \
  99477f4492b77bcb746000b4633c2cc255e203dd \
  c61300cacdf9bf6b91cad8b7864b33cfbd03ecfe
# EXIT=1  (tip é DESCENDENTE, não ancestral)

# É o headRefOid (c61300ca) ancestral do tip (99477f4a)?
git merge-base --is-ancestor \
  c61300cacdf9bf6b91cad8b7864b33cfbd03ecfe \
  99477f4492b77bcb746000b4633c2cc255e203dd
# EXIT=0  (headRefOid é ancestral do tip → case 2 → keep)
```

Para SHA_B completamente não relacionado com SHA_A: `is-ancestor(SHA_B, SHA_A)` retorna false
(objetos não relacionados) → case 2b → keep.

**Veredito**: **coberto pelo ADR**. D1 step 0 (OPEN PR) é guarda primária. Sem OPEN PR, o
`is-ancestor` retorna false para qualquer tip que divergiu de SHA_A. O único subcase em que case 1
dispara: SHA_B é ancestral de SHA_A (developer voltou a um commit mais antigo). Nesse caso, SHA_B
está contido no PR mergeado → delete é correto.

---

### Cenário 3 — Vários PRs com o mesmo head (um MERGED, um OPEN)

**Medição**:

```bash
gh pr list --state all --limit 500 \
  --json number,state,headRefName \
  | python3 -c "
import json,sys
from collections import defaultdict
d=json.load(sys.stdin)
groups=defaultdict(list)
for p in d: groups[p['headRefName']].append(p)
multi=[g for g in groups.values() if len(g)>1]
print('Branch names with multiple PRs:', len(multi))
for g in multi[:2]:
    print(' '+g[0]['headRefName']+': '+', '.join('#%d %s' % (p['number'],p['state']) for p in g))
"
# Branch names with multiple PRs: 6
#  fix/fechar-os-grupos-de-falha-de-windows...: #311 MERGED, #304 MERGED, #270 MERGED, #269 MERGED, #267 MERGED
#  fix/v8-um-binario-muitos-canais: #365 MERGED, #361 MERGED, #356 MERGED, #355 MERGED, #354 MERGED, #352 MERGED
# EXIT=0
```

Não há caso OPEN+MERGED no acervo — não medido com fixture real. A lógica: D1 step 0 verifica
OPEN PRs primeiro. Se algum OPEN PR existir (não fork) → keep, independentemente de quantos MERGED
existirem. Se vários MERGED: "basta que um contenha o tip".

**Veredito**: **coberto pelo ADR** (D1 step 0, case 1). Nenhum ajuste.

---

### Cenário 4 — Resposta truncada ou paginada do `gh pr list`

**Medição**:

```bash
time gh pr list --state all --limit 3000 --json number --jq length
# 428
# real 0m2.332s
# EXIT=0
```

428 PRs, 2.3 s. Com `--limit 500` neste repositório: 428 < 500, sem truncamento. D3 detecta
truncamento por `items == limit`; com 500 e 428 PRs, o alarme não dispara incorretamente.

**Risco: OPEN PR fora da janela com branch content-idêntica**. D3 cobre branches **ausentes** da
resposta truncada (fallback para content heuristic). Não cobre o caso em que a branch está presente
com um MERGED PR na janela, mas seu OPEN PR mais recente ficou fora. Nesse caso, D1 step 0 é
ignorado e case 1 pode disparar. A branch seria apagada, mas o OPEN PR ainda existe.

**Veredito D3**: **requer ajuste**. Sob truncamento confirmado (`items == limit`), o sinal de PR não
deve autorizar `delete`, apenas `keep` ou `review`. O delete sob truncamento deve ser bloqueado
inteiramente. Isso é mais restritivo que D3 conforme escrito ("branch ausente fica no veredito de
hoje"), mas é a única garantia correta. Caso o arquiteto prefira aceitar este como resíduo em vez
de ajuste, a justificativa é: um OPEN PR sempre mantém a branch remota viva, e o delete perde
apenas a cópia local, recuperável via `git fetch origin refs/pull/N/head` (medido abaixo).

Sobre o valor de N: com 428 PRs e 2.3 s, `--limit 1000` é seguro e deixa margem de ~2.3× antes do
alarme de truncamento disparar. `--limit 500` deixa apenas 17% de margem; recomendado mínimo 1000.

---

### Cenário 5 — `gh` resolvendo repositório a partir de outro remoto ou host; `GH_REPO` misdirecionado

**Medição — multi-remoto**:

```bash
# scratch: origin=kgsaran/trackfw, upstream=cli/cli
git init -q /tmp/multi-remote-test
cd /tmp/multi-remote-test
git remote add origin git@github.com:kgsaran/trackfw.git
git remote add upstream https://github.com/cli/cli.git

gh repo view --json nameWithOwner --jq .nameWithOwner
# cli/cli
# EXIT=0

gh pr list --state all --limit 2 --json number,headRefName
# PRs de cli/cli
# EXIT=0
```

O `gh` resolveu para `cli/cli`, não para `kgsaran/trackfw`. (O repositório escolhido correspondeu
ao segundo remoto adicionado; não foi testado se o comportamento é "último adicionado" ou
"preferência por remoto nomeado `upstream`" — apenas o resultado foi observado.)

**Medição — `GH_REPO` override**:

```bash
GH_REPO=cli/cli gh pr list --state all --limit 2 --json number,headRefName
# [{"headRefName":"fix/display-url-escaped-path","number":14586}, ...]
# EXIT=0

GH_REPO=torvalds/linux gh pr list --state all --limit 2 --json number,headRefName
# []
# EXIT=0

# --repo sobrescreve GH_REPO
GH_REPO=torvalds/linux gh pr list --repo kgsaran/trackfw \
  --state all --limit 2 --json number,headRefName
# [{"headRefName":"chore/fecha-req-2026-09-01-contributing","number":513}, ...]
# EXIT=0
```

**Medição — `GH_HOST` mismatch**:

```bash
GH_HOST=github.example.com gh pr list --state all --limit 1
# none of the git remotes configured...correspond to the GH_HOST env variable
# EXIT=1
```

**Análise do risco**: `GH_REPO=cli/cli` retorna exit 0 com dados de outro repositório. Para repos
completamente não relacionados (cli/cli vs trackfw), objetos de headRefOid não existem localmente →
`cat-file -e` falha → `review`. Mas no caso de multi-remoto onde origin é um fork do mesmo projeto
(fork-workflow), os objetos são compartilhados. Nesse cenário, `is-ancestor` pode passar com o PR
de um colaborador cujo fork foi mergeado — e deletar a branch local, cujo conteúdo está em main de
qualquer forma.

**Mitigation**: o ML-1A deve passar `--repo HOST/OWNER/REPO` derivado de `git remote get-url origin`
para `gh pr list`. O flag `--repo` sobrescreve `GH_REPO` (medido). O host deve ser `github.com`
(verificado pela função `extractHost` já existente em `internal/forge/resolve.go:111`). Caso
contrário, degradar (D2). **Não usar `forge.Resolve`**: seu fallback de CI retorna `"github"` se
`.github/workflows/` existir, mesmo com origin apontando para outro host.

**Veredito**: **requer ajuste**. O ML-1A deve passar `--repo HOST/OWNER/REPO` derivado de
`git remote get-url origin` para `gh pr list`.

---

### Cenário 6 — Erro com exit 0, ou JSON parcial

**Medição**:

```bash
# Sem autenticação
GH_CONFIG_DIR=/tmp/empty-gh-config GH_TOKEN= gh pr list --state all --limit 2 --json number
# To get started with GitHub CLI, please run: gh auth login
# EXIT=4

# GH_HOST mismatch
GH_HOST=github.example.com gh pr list --state all --limit 1
# none of the git remotes configured...
# EXIT=1
```

Exit ≠ 0 em ambos os casos de erro genuíno de auth/host → D2 detecta corretamente.

Caso residual: `GH_REPO=cli/cli` com exit 0 e JSON válido — coberto pelo Cenário 5 via `--repo`
explícito. JSON parcialmente truncado (output interrompido): `json.Unmarshal` falha → tratar como
D2, não como lista vazia.

**Veredito**: **requer ajuste**. O ML-1A deve tratar qualquer exit ≠ 0 E qualquer falha de parse
JSON como D2. Lista vazia `[]` com exit 0 é resposta legítima.

---

### Cenário 7 — Branch sem upstream

**Medição**:

```bash
# Branch nunca empurrada
git for-each-ref \
  --format="%(refname:short) upstream=%(upstream:short) track=%(upstream:track)" \
  refs/heads/fix/branch-prune-consulta-o-estado-do-pr
# fix/branch-prune-consulta-o-estado-do-pr upstream= track=
# EXIT=0

# Branch com upstream gone (mergeada, remoto apagado)
git for-each-ref \
  --format="%(refname:short) upstream=%(upstream:short) track=%(upstream:track)" \
  refs/heads/chore/fecha-req-2026-09-01-contributing
# chore/fecha-req-2026-09-01-contributing upstream=origin/chore/... track=[gone]
# EXIT=0

# git rev-parse <branch>@{u} em upstream gone: não funciona
git rev-parse --abbrev-ref "chore/fecha-req-2026-09-01-contributing@{u}"
# fatal: ambiguous argument '...@{u}': unknown revision or path...
# EXIT=128

# git rev-parse @{u} (bare, usado por buildPushArgs): apenas a branch atual
git rev-parse --abbrev-ref --symbolic-full-name "@{u}"
# fatal: no upstream configured for branch 'fix/branch-prune-consulta-o-estado-do-pr'
# EXIT=128
```

`%(upstream:short)` vazio → nunca empurrada (step 4). Não-vazio (incluindo `[gone]`) → teve
upstream (step 5). O `git rev-parse <branch>@{u}` falha em branches com `[gone]` (EXIT=128), e o
`git rev-parse @{u}` bare de `buildPushArgs` resolve apenas a branch atual — ambos são predicados
errados para esta finalidade.

**Veredito**: **requer ajuste**. O ML-1A deve usar `git for-each-ref --format='%(upstream:short)'`
para detectar "nunca empurrada"; campo vazio = step 4. Não copiar `buildPushArgs`/`git rev-parse @{u}`.

---

### Cenário 8 — `headRefOid` do PR mergeado por squash: ainda é o tip que o autor empurrou?

**Medição**:

```bash
# PR #513: squash-merged em c9c1c7bc
gh pr view 513 --json headRefOid --jq .headRefOid
# d597d742f1705c777e146dec103a78d281ef4cdf   (commit do autor)

git rev-parse chore/fecha-req-2026-09-01-contributing
# d597d742f1705c777e146dec103a78d281ef4cdf   (idêntico → tip == headRefOid)

# headRefOid NÃO está em main (squash cria novo commit com SHA diferente)
git merge-base --is-ancestor d597d742f1705c777e146dec103a78d281ef4cdf main
# EXIT=1

# headRefOid ausente depois que objeto pode ser coletado pelo GC
# Detectar ausência: cat-file -e, não erro de is-ancestor
git cat-file -e d597d742f1705c777e146dec103a78d281ef4cdf
# EXIT=0 (presente)

# Recoverability: GitHub mantém refs/pull/N/head permanentemente
git ls-remote origin refs/pull/513/head
# d597d742f1705c777e146dec103a78d281ef4cdf   refs/pull/513/head
# EXIT=0

git ls-remote origin refs/pull/100/head
# 70c9409bac16a42ee0df8c249d5d09e64925f07a   refs/pull/100/head
# EXIT=0
```

`headRefOid` é o tip que o autor empurrou, congelado no momento do merge. O squash commit em main
tem SHA diferente. Após GC, o headRefOid pode ser coletado se a branch local que apontava para ele
foi apagada. Populações em risco: (a) o autor empurrou/amend de outra máquina antes do merge — o
tip local difere do headRefOid, e o headRefOid nunca foi baixado neste clone; (b) clone fresh sem
`git fetch origin refs/pull/N/head`.

`git merge-base --is-ancestor tip absent_oid` retorna exit 128 (fatal), indistinguível de exit 1
(not an ancestor) pelo valor de retorno do erro genérico de `defaultGitExec`. Distinguir por texto
do erro é frágil. O `git cat-file -e <oid>^{commit}` antes de `is-ancestor` é o caminho correto.

**Veredito**: **requer ajuste**. O ML-1A deve executar `git cat-file -e <oid>^{commit}` antes de
`is-ancestor`; ausência → `review`, não `delete`, conforme D1.

---

### Cenário 9 — `baseRefName`: PR mergeado em branch que não é `main`

**Medição neste repositório**:

```bash
gh pr list --state all --limit 3000 \
  --json number,baseRefName,state \
  --jq "[.[] | select(.baseRefName != \"main\")] | length"
# 0
# EXIT=0
```

Zero PRs com base diferente de `main`. Mas em repositórios com stacked PRs: PR A mergeado em
`feature/v2` (não `main`). headRefOid de PR A passa `is-ancestor` → case 1 → delete. O trabalho
está em `feature/v2`, não em `origin/main`. A branch seria apagada incorretamente.

O content heuristic detectaria divergência (diverg não vazio em relação a `origin/main`) e diria
`pending_work` — mas o sinal de PR é mais forte no design D1 e substituiria o content heuristic.

**Veredito**: **requer ajuste**. O ML-1A deve incluir `baseRefName` no payload de `gh pr list` e
exigir `baseRefName == defaultBranch` (resolvido de `branchPruneDefaultLocalName = "main"`) no
filtro de case 1. Um PR mergeado em branch não-default não autoriza delete.

---

### Cenário 10 — D4: novos vereditos e o aviso do `push`/`ship`; prefixo `origin/`

**Código atual** (`ship.go:786–805`):

`detectPendingSquashMerges` lista branches remotas com `git branch -r --no-merged origin/main` e
passa cada uma — na forma `origin/X` — para `evaluateBranchIntegration`. O aviso dispara apenas
para `branchPruneDecisionPendingWork`.

**Problema 1 — prefixo `origin/`**: após ML-1A, a função de lookup de PR usará `headRefName`
(o nome curto da branch). Se o match for feito com o argumento `origin/X`, nenhum PR casará. A
implementação passaria para o step 5 (sem PR, com upstream → content heuristic). O sinal de PR
nunca chegaria ao aviso do push/ship. Os steps 4/5 também não se aplicam a refs remotas (uma ref
`origin/X` não tem "upstream" no sentido de `%(upstream:short)`).

**Problema 2 — novos vereditos**: após ML-1A, branches com case 2 (tip além do head do PR) e case
2b (tip divergiu) são genuinamente não integradas em um PR novo. Ambas devem disparar o aviso.
Branches com case 0 (OPEN PR) não devem (o PR já está aberto — aviso seria redundante). Se o filtro
de `detectPendingSquashMerges` não for atualizado para as novas constantes, o aviso silencia para
os casos 2 e 2b: falso negativo no aviso.

**Veredito**: **requer ajuste**. O ML-1A deve: (a) no lado do aviso, usar o nome curto (`shortName`
já existe em `ship.go:797`) para o match de PR, nunca a forma `origin/X`; (b) declarar quais novas
constantes disparam o aviso — mínimo: case 2, case 2b, `branchPruneDecisionPendingWork`; excluir
case 0; (c) a fixture de AC6 deve cobrir o caminho `origin/<branch>`.

---

### Cenário 11 — Verificação independente do split 49/3

**Medição**:

```bash
PR_JSON=$(gh pr list --state all --limit 500 \
  --json number,state,headRefName,headRefOid,isCrossRepository)
# 428 PRs, EXIT=0

mapfile -t BRANCHES < <(git for-each-ref --format="%(refname:short)" refs/heads)
# 54 branches
```

Resultado por branch (saída completa):

```
DELETE: chore/fecha-req-2026-09-01-contributing
DELETE: chore/fecha-req-364
DELETE: chore/fecha-req-451
DELETE: chore/fecha-req-471-criterio-de-adr-por-prefixo
DELETE: chore/fecha-req-476-cerca-nao-terminada
DELETE: chore/fecha-req-485-gate-reentrante-barrier
DELETE: chore/fecha-req-491-gate-por-linha
DELETE: chore/fecha-req-494-490-estado-que-governa-a-branch
DELETE: chore/fecha-req-504-limite-por-chunk
DELETE: chore/fecha-req-507-guard-sem-jq
DELETE: chore/release-v9-0-0
DELETE: chore/release-v9-0-1
DELETE: chore/roadmaps-faltantes-governanca
DELETE: docs/a-apuracao-do-censo-...
DELETE: docs/amplia-req-rastreabilidade-com-o-req-move
CASE2:  docs/bash-consome-stdout-de-python3-sem-normalizar-crlf
DELETE: docs/caminho-posix-interpolado-...
DELETE: docs/contributing-regras-de-contribuicao
DELETE: docs/fecha-req-guarda-de-folha-contencao
DELETE: docs/fecha-req-vinculo-req-roadmap-pos-merge-446
CASE2:  docs/fechamento-req-2026-09-28-stale-roadmap-para-req
DELETE: docs/fechamento-req-387
DELETE: docs/fechamento-req-435-orphan-req
DELETE: docs/fechamento-req-445-init-destroi-config
DELETE: docs/fechamento-req-450-e-contencao-windows
DELETE: docs/treze-rotulos-falham-no-censo-...
DELETE: fix/a-apuracao-do-censo-...
DELETE: fix/afirma-contencao-antes-de-escrever
DELETE: fix/barrier-executa-cada-linha-do-bloco-de-gates
DELETE: fix/bash-consome-stdout-de-python3-sem-normalizar-crlf
NO_PR:  fix/branch-prune-consulta-o-estado-do-pr
DELETE: fix/caminho-posix-interpolado-...
DELETE: fix/cerca-nao-terminada-mascara-em-silencio
DELETE: fix/contencao-nao-ve-juncao-do-windows
DELETE: fix/context-reporta-zero-adrs-onde-status-reporta-145
CASE2:  fix/criterio-de-adr-por-prefixo
DELETE: fix/dois-workflows-rodam-a-mesma-validacao
DELETE: fix/estado-que-governa-a-branch
DELETE: fix/gate-de-palavra-chave-...
DELETE: fix/gate-de-wave-que-reentra-no-barrier-recursa-sem-limite
DELETE: fix/guard-de-branch-falha-aberto-sem-jq
DELETE: fix/guarda-de-folha-resolve-o-caminho-...
DELETE: fix/init-e-discover-geram-dois-workflows
DELETE: fix/init-reexecutado-destroi-config-do-consumidor
DELETE: fix/leniencia-sem-prazo
DELETE: fix/limite-de-tempo-por-chunk-na-falsificacao
DELETE: fix/literal-embutido-nao-normaliza-crlf-e-o-gate-nao-o-varre
DELETE: fix/orphan-req-reprova-estado-correto
DELETE: fix/req-nasce-orfa-porque-criar-req-e-criar-roadmap-sao-dois-comandos
DELETE: fix/roadmap-para-req-sem-tratamento-de-stale
DELETE: fix/scaffold-placeholder-chega-a-done
DELETE: fix/teste-e-gate-leem-a-arvore-de-governanca
DELETE: fix/treze-rotulos-falham-no-censo-...
NO_PR:  main
```

**Resultado**: 49 DELETE, 3 CASE2, 0 CASE2B, 2 NO_PR (main e fix/branch-prune-consulta-o-estado-do-pr
— branch atual, nunca empurrada). O split do arquiteto (49 delete, 3 keep) está correto.

As 3 branches CASE2 são as 3 branches que o ADR menciona na seção Consequences:
`docs/bash-consome-stdout-de-python3-sem-normalizar-crlf`,
`docs/fechamento-req-2026-09-28-stale-roadmap-para-req` e
`fix/criterio-de-adr-por-prefixo` (9 commits além do headRefOid do PR #503).

---

## Seção 3 — Alvos de falsificação nas duas direções

**Os dois sabotagens mais prováveis para o Wave 2 (ML-2A)**:

**Sabotagem 1 — Inversão de argumentos em `is-ancestor`**

A chamada correta é `is-ancestor <tip> <headRefOid>`. A inversão — `is-ancestor <headRefOid> <tip>` — verifica se `headRefOid` é ancestral do `tip`. Para branches com commits além do headRefOid (case 2), isso retorna `EXIT=0`:

```bash
# fix/criterio-de-adr-por-prefixo: case 2, tip TEM commits além do headRefOid
git merge-base --is-ancestor \
  c61300cacdf9bf6b91cad8b7864b33cfbd03ecfe \  # headRefOid
  99477f4492b77bcb746000b4633c2cc255e203dd     # tip
# EXIT=0  ← ERRADO: a ordem invertida diz "delete" quando é "keep" (case 2)
```

Com a ordem invertida, as 3 branches de CASE2 seriam deletadas. Estas são as únicas branches com
trabalho não integrado no acervo real. `fix/criterio-de-adr-por-prefixo` é a fixture natural para
AC9.

**Sabotagem 2 — Não passar `--repo`; depender de `GH_REPO` ambiental**

Sem `--repo`, qualquer ambiente com `GH_REPO` ou múltiplos remotos pode consultar o repositório
errado. Com repos completamente não relacionados, `cat-file -e` falha → `review` (safe). Com repos
relacionados (fork do mesmo projeto), objetos compartilhados → `is-ancestor` pode passar para PRs
certos mas de fork → delete de branch local com trabalho que foi mergeado via fork → correto, mas
por acidente. O risco real: repositório errado devolve `[]` ou lista com headRefOids ausentes → 49
branches ficam sem sinal de PR → fallback para content heuristic. Com veredito hoje de "1 delete,
25 review, o resto keep", essas 49 voltariam para o veredito atual, que é exatamente o defeito que
a REQ existe para corrigir — falso negativo massivo.

**Tabela de falsificação por cenário**:

| Cenário | Controle que fecha o FP (delete indevido) | Sabotagem para FP | Controle que fecha o FN (não-delete de mergeada) | Sabotagem para FN |
|---|---|---|---|---|
| C1: Fork | `is-ancestor` + `baseRefName` (o fork PR pode ter conteúdo em main) | Não é um controle de FP — fork não apaga indevidamente | `isCrossRepository: false` + `--repo` | Não checar fork |
| C2: Branch reaproveitada | `is-ancestor(tip, headRefOid)` = false para tip divergente | Inverter argumentos (sabotagem 1) | D1 step 0 (OPEN PR) | Não verificar OPEN PR antes de MERGED |
| C3: Vários PRs | D1 step 0 (OPEN > MERGED) | Não checar OPEN PRs | `any merged contains tip` | Exigir que TODOS os MERGED contenham o tip |
| C4: Truncamento | Bloquear `delete` inteiramente sob truncamento | Não checar truncamento / usar `==` sem bloquear delete | N alto o suficiente | N muito baixo (ex. 100) |
| C5: Repositório errado | `--repo HOST/OWNER/REPO` explícito (sabotagem 2) | Não passar `--repo` | `forge.Resolve` não usado | Usar `forge.Resolve` (CI-file fallback) |
| C6: Erro JSON exit 0 | Parse error → D2 | Tratar parse error como `[]` | Exit ≠ 0 → D2 | Checar só exit, não parse |
| C7: Sem upstream | `%(upstream:short)` vazio → step 4 | Copiar `git rev-parse @{u}` | Upstream vazio → keep | Tratar `[gone]` como "sem upstream" |
| C8: headRefOid ausente | `cat-file -e` antes de `is-ancestor` | Tratar exit 128 como exit 1 | `review` quando objeto ausente | Degradar para `delete` quando ausente |
| C9: base != main | `baseRefName == defaultBranch` | Não incluir `baseRefName` na query | Sem residual — content heuristic diz `pending_work` para trabalho em feature branch | Ignorar content heuristic quando PR presente |
| C10: D4 mapping | `shortName` (não `origin/X`) no match; novas constantes no filtro | Usar `origin/X` no match; copiar filtro antigo | Cases 2/2b → aviso | Caso 0 (OPEN PR) → aviso redundante |

---

## Seção 4 — Resíduo declarado

**R1 — Forges além do GitHub**: D2 declara explicitamente. GitLab, Azure, Bitbucket degradam para o
veredito de hoje. Não é regressão.

**R2 — PR mergeado por rebase com reescrita total de commits**: headRefOid não existe como objeto
acessível (reescrita antes do merge). `cat-file -e` falha → `review`. Humano decide.

**R3 — Fork-workflow consumers**: consumidores cujo `origin` é seu fork do projeto principal
precisam configurar `--repo upstream/repo` para obter o sinal de PRs do repositório principal.
Este design não cobre fork-workflow sem intervenção do consumidor.

**R4 — Listagem de branch por `\n` sem `-z`**: herdado do REQ-2026-08-18, documentado na revisão
de 2026-08-18. Criar ref com `\n` via `update-ref` exige o mesmo nível de acesso que apagar
branches diretamente. Não é escalação de privilégio.

**R5 — `headRefOid` de PR ausente neste clone**: o objeto `headRefOid` pode nunca ter sido baixado
se o autor empurrou ou emendou de outra máquina antes do merge, e o clone atual nunca fez `git
fetch origin refs/pull/N/head`. Resultado: `cat-file -e` falha → `review`. Comportamento
conservador. O objeto pode ser obtido explicitamente via `git fetch origin refs/pull/N/head`
(medido: GitHub mantém esses refs permanentemente). Nota: PRs cujas branches **locais** foram
apagadas nunca chegam a `evaluateBranchIntegration` (o prune só avalia branches locais existentes).

**R6 — OPEN PR fora da janela de N itens**: se o sinal de PR é bloqueado inteiramente sob
truncamento (ajuste de C4), esta classe vira resíduo: a branch fica no veredito do content
heuristic, não `delete`. Caso o ajuste de C4 não seja aceito e o delete permaneça possível sob
truncamento, este resíduo é: OPEN PR sempre mantém a branch remota viva; a cópia local é
recuperável via `refs/pull/N/head`. A mitigação prática é N ≥ 1000.

---

## Resumo de ajustes para o ML-1A

| # | Ajuste | Prioridade |
|---|---|---|
| A1 | Passar `--repo HOST/OWNER/REPO` derivado de `git remote get-url origin` para `gh pr list`; verificar host == `github.com`; não usar `forge.Resolve` | Crítico |
| A2 | Incluir `baseRefName` no payload; filtrar case 1 por `baseRefName == defaultBranch` ("main") | Alto |
| A3 | Executar `git cat-file -e <oid>^{commit}` antes de `is-ancestor`; ausência → `review` | Alto |
| A4 | Detectar upstream com `%(upstream:short)` via `for-each-ref`; campo vazio = step 4; não copiar `git rev-parse @{u}` | Alto |
| A5 | Tratar qualquer exit ≠ 0 E qualquer falha de parse JSON como D2; lista vazia `[]` com exit 0 é resposta legítima | Médio |
| A6 | No aviso de `detectPendingSquashMerges`: (a) usar `shortName` para match de PR, nunca `origin/X`; (b) declarar quais constantes disparam — mínimo cases 2, 2b, `PendingWork`; excluir case 0; (c) a fixture de AC6 deve cobrir o caminho `origin/<branch>` | Médio |
| A7 | Bloquear `delete` quando truncamento confirmado (`items == limit`); ou declarar R6 como resíduo aceito | Médio |
| A8 | Usar `--limit 1000` (ou maior) para deixar margem antes do alarme de truncamento | Baixo |
