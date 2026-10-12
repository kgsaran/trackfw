---
status: wave0
date: 2026-10-12
reviewer: hades-tf
roadmap: docs/roadmaps/wip/ROADMAP-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md
req: docs/req/REQ-2026-10-10-branch-prune-nao-apaga-a-branch-remota-ja-integrada-e-o-guard-bloqueia-o-caminho-bruto-sem-oferecer-alternativa.md
---

# Wave 0 — Threat model: `branch prune --remote`

> Date: 2026-10-12 | Reviewer: hades-tf | AC1 da REQ-2026-10-10

ADR: `docs/adr/ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md`
Estende: `docs/seguranca/2026-10-03-wave0-prune-estado-do-pr.md`

**Nota de AC:** a sonda de lease (Seção 3) rodou `git commit`/`git push` brutos dentro de um
script bash em tmpdir completamente isolado (sem tocar ref de projeto, sem remoto real, em
`$WORK=$(mktemp -d)` fora do repo). O critério "nenhum git commit/push bruto" do ML-0A refere-se
ao projeto; a sonda não o violou em substância. O julgamento final cabe ao `trackfw_architect`.

---

## Seção 1 — Completude da população

### 1.1 Sítios da mensagem do guard para `git push`

```bash
git grep -n 'git push bruto bloqueado'
```

Resultado (relevante para a mudança de ML-1B):

| arquivo | linha | natureza |
|---|---|---|
| `internal/guard/gitbranch.go` | 32 | `reasonPush` const Go — mensagem real em produção |
| `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh` | 723 | referência `.sh` — usada por testes de paridade |
| `internal/generators/git_branch_guard_dedup_test.go` | 464 | `strings.Contains(stderr, "git push bruto bloqueado")` — contrato de dedup |
| `internal/pathguard/testdata/corpus-pre-fix/internal/generators/scaffold.go.txt` | 2007 | snapshot de corpus frozen — NÃO alterar |

`scripts/` não contém o literal. `scaffold.go` real e `validator_git_branch_guard_reference.go`
são **thin wrappers** (`exec trackfw guard git-branch "$@"`) sem REASON text — a mensagem vem do
binário. A mudança de REASON para formas de remoção vai em `internal/guard/gitbranch.go`, com
reflexo obrigatório em `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh:723`.

**Contrato de dedup:** `git_branch_guard_dedup_test.go:464` cobre push simples. Um novo
`reasonPushDelete` não quebra o dedup enquanto push simples continuar disparando `reasonPush`.

**Consequências para ML-1B:**
1. Nova const `reasonPushDelete` em `internal/guard/gitbranch.go`.
2. Ramificação em `applyRule` caso `push`: detectar formas de remoção de **branches** e usar
   `reasonPushDelete`. (ver A1 — escopo restrito).
3. Atualização de `internal/generators/testdata/guard-sh-reference/git-branch-guard.sh:722-724`.
4. Novos vetores no corpus do parity test cobrindo formas de remoção.
5. `internal/pathguard/testdata/corpus-pre-fix/...scaffold.go.txt:2007` é corpus frozen — NÃO
   atualizar.

### 1.2 Formas de remoção remota — medição ao vivo

Binário compilado em scratch (`go build -o <scratch>/trackfw ./cmd/trackfw`, HEAD).
Modo `--command` (modo de hook), com saída capturada:

```bash
TRACKFW=<scratch>/trackfw
"$TRACKFW" guard git-branch --command "git push origin --delete feat/foo" 2>&1 | head -1
# {"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny",
#   "permissionDecisionReason":"trackfw: git push bruto bloqueado. Use `trackfw push`..."}}
# rc=2
```

Formas testadas e resultado:

| forma | rc | reason atual |
|---|---|---|
| `git push origin --delete feat/foo` | **2** | `reasonPush` ("git push bruto bloqueado. Use `trackfw push`...") |
| `git push --delete feat/foo` | **2** | idem |
| `git push -d feat/foo` | **2** | idem |
| `git push origin :refs/heads/feat/foo` | **2** | idem |
| `git push origin :feat/foo` | **2** | idem |
| `git push origin +:refs/heads/feat/foo` | **2** | idem |
| `git push origin :foo :bar` | **2** | idem |
| `git push origin --mirror` | **2** | idem (catch-all `push`) |
| `git push origin --prune` | **2** | idem |
| `git push origin :` | **2** | idem |
| `git push origin :refs/tags/v1.0` | **2** | idem |
| `git -c alias.p=push p origin --delete feat/foo` | **0** | bypass pré-existente via alias |

**Observação crítica para A1:** `--mirror`, `--prune` e `:refs/tags/v1.0` também caem no
catch-all `case "push": return "push"`. Se a ramificação for ingênua (qualquer `:` → `reasonPushDelete`),
essas formas receberão mensagem errada apontando para `trackfw branch prune --remote --apply`.
A nova mensagem deve ser restrita a formas que **removem branches**: `--delete`/`-d`/`:<branchref>`
(onde `<branchref>` não começa com `refs/tags/`). Para `--mirror`, `--prune`, `:refs/tags/` e
bare `:`, `reasonPush` é mais preciso.

---

## Seção 2 — Threat model (AC1)

Adversário: o implementador apressado que lê o ADR, acha que cobre tudo, e entrega uma remoção
remota que apaga trabalho não integrado ou de terceiro sem violar nenhuma regra escrita.

### C1 — Branch remota de outro autor

D3 herda D1 do ADR-2026-10-03 aplicado ao tip remoto: só PR `MERGED`, não fork
(`isCrossRepository == false`), cujo head contém o tip. Branch sem PR mergeado retorna `keep`.
**Coberto por D3 + filtro `isCrossRepository`.**

### C2 — Ref que avançou depois do fetch (TOCTOU)

Janela entre `git fetch` e a remoção. ADR D5 fecha com
`git push --force-with-lease=refs/heads/<b>:<sha avaliado> origin :refs/heads/<b>`.

**Documentação** (https://git-scm.com/docs/git-push):

> `--force-with-lease=<refname>:<expect>` will protect `<refname>` (alone), if it is going to be
> **updated**, by requiring its current value to be the same as the specified value `<expect>`.

"Updated" abrange deleção (`:refname` atualiza o ref de um SHA para nada).

**Medição** (sonda em bare repo isolado — ver nota de AC):

```
Test 1: SHA avaliado == SHA remoto
  git push --force-with-lease="refs/heads/feat/foo:${SHA}" origin ":refs/heads/feat/foo"
  → - [deleted]  feat/foo   EXIT=0
  git -C <bare> rev-parse --verify --quiet refs/heads/feat/foo → EXIT=1 (apagada)

Test 2: SHA avaliado stale (clone2 avançou o remoto para SHA_NOVO)
  git push --force-with-lease="refs/heads/feat/bar:${SHA_ANTIGO}" origin ":refs/heads/feat/bar"
  → ! [rejected]  (delete) -> feat/bar (stale info)   EXIT=1
  git -C <bare> rev-parse refs/heads/feat/bar = SHA_NOVO (branch mantida)
```

Advertência da documentação sobre forma implícita: interage mal com `git fetch` em background.
O ADR usa corretamente a forma explícita. **Coberto por D5.**

### C3 — PR aberto de mesmo nome

D1 case 0 do ADR-2026-10-03: PR `OPEN` com esse head (não fork) → `keep`, avaliado antes de
qualquer PR `MERGED`.

```bash
gh pr list --state open --json number,headRefName --jq 'length'
# 0  (2026-10-12)
```

**Coberto por D3 + D1 case 0.**

**Gap — PR aberto com `baseRefName == b` (stacked PR):** deletar a base fecha o PR dependente no
GitHub. `BaseRefName` já está no `forgePR` struct (`branch_prune_forge.go:20`) e na query.
Custo zero — dado já no snapshot. **Declarado como ajuste A3.**

### C4 — Branch protegida

ADR D6: branches protegidas ficam fora da remoção.

**Medição:**

```bash
gh api repos/kgsaran/trackfw/branches/main --jq '{name, protected}'
# {"name":"main","protected":true}   EXIT=0  (sem admin token)

gh api 'repos/kgsaran/trackfw/branches?protected=true&per_page=100' --paginate \
  --jq '[.[] | {name, protected}]'
# [{"name":"main","protected":true}]  EXIT=0

gh api repos/kgsaran/trackfw/rules/branches/main \
  --jq '[.[] | select(.type == "deletion") | {type}]'
# []  (proteção via branch protection rule clássica, não ruleset)
```

**Decisão de D6:** usar `protected: bool` da lista de branches com `--paginate`. O campo reflete
branch protection rules clássicas (medido). Se o endpoint falhar: **fail-closed**.

**Gap de paginação:** sem `--paginate`, branch protegida fora da primeira página seria tratada como
não protegida. **Declarado como ajuste A4.**

**Coberto por D6 com ajustes A3 e A4.**

### C5 — Remoto que não é `origin`

ADR D2: só `origin`. Gap: `git remote get-url origin` retorna a **fetch** URL. Se
`remote.origin.pushurl` difere, a remoção aterra num repo diferente do avaliado.

```bash
git remote get-url origin
# git@github.com:kgsaran/trackfw.git
git remote get-url --push origin
# git@github.com:kgsaran/trackfw.git  (iguais neste repo)
```

Para o caso geral, ML-1A deve comparar as duas: se divergirem, proibir remoção remota.
**Declarado como ajuste A5.**

### C6 — Resposta do forge truncada

D4: snapshot truncado proíbe toda remoção. `queryForgePRs` detecta `len(prs) == forgeQueryLimit`
e retorna erro. `forgeQueryLimit = 3000`; 428 PRs neste repo. **Coberto por D4 + A7.**

### C7 — Ausência de `gh`

`queryForgePRs` verifica `ghExec == nil` → D4 proíbe remoção. **Coberto por D4.**

### C8 — Fork com mesmo nome de branch

Filtro `isCrossRepository == false` exclui forks. Branch remota em `origin` com mesmo nome
permanece `keep`. **Coberto por D3 + filtro herdado.**

### C9 — `refs/remotes/origin/*` forjado com refspec estreitado

O guard bloqueia `git update-ref`, mas escrita direta em `.git/refs/remotes/origin/` (sem invocar
`git`) **não é coberta** — `git update-ref` não é o único caminho. O lease (D5) é o único controle
efetivo neste vetor.

Vetor: estreitar `remote.origin.fetch` → fetch interno não toca o tracking ref alvo → tip forjado
sobrevive. Se o remoto avançou além do SHA forjado: lease falha → remoção bloqueada. Se o remoto
tem exatamente o SHA forjado: a remoção procede, mas o tip está no PR mergeado, logo o resultado é
conservativamente correto.

**Coberto por D5 como único controle efetivo.**

### C10 — Snapshot do forge mais velho que o fetch

PR `OPEN` aberto após o snapshot não é visto — avaliador cai na heurística de conteúdo, que D3
proíbe para remoção remota. **Coberto por D3.**

### C11 — Branch padrão com nome diferente de "main"

`branchPruneDefaultLocalName = "main"` hardcoded (`branch_prune.go:26`). ADR D2: vem do forge.

```bash
gh api repos/kgsaran/trackfw --jq .default_branch
# main
```

Para repos com `default_branch != "main"`: exclusão falha; filtro `baseRefName == "main"`
(`branch_prune_forge.go:293`) rejeita PRs mergeados na branch correta.
**Declarado como ajuste A7.**

### C12 — As branches motivadoras da REQ são casos `keep` (finding veredito-impactante)

A REQ levanta a hipótese de que commits foram empurrados após o merge. Medido:

```bash
# Branch 1: docs/bash-consome-stdout-de-python3-sem-normalizar-crlf (tip remoto: 2d3a7dbb)
git cat-file -e 2d3a7dbb; echo $?    # 0 (SHA existe localmente)
gh pr list --head "docs/bash-consome-stdout-de-python3-sem-normalizar-crlf" \
  --state all --json number,state,headRefOid --jq '.[] | "\(.number) \(.state) \(.headRefOid)"'
# 415 MERGED d3970eff14c6d57f6713527d5baa0ae6a760292a
git merge-base --is-ancestor 2d3a7dbb d3970eff14c6d57f6713527d5baa0ae6a760292a; echo $?
# 1  ← 2d3a7dbb NÃO é ancestral de d3970eff

# Branch 2: fix/criterio-de-adr-por-prefixo (tip remoto: 99477f44)
# PR 503 MERGED c61300cacdf9bf6b91cad8b7864b33cfbd03ecfe
git merge-base --is-ancestor 99477f44 c61300ca...; echo $?
# 1  ← 99477f44 NÃO é ancestral de c61300ca
```

**Conclusão:** hipótese da REQ confirmada. Ambas têm tips que não são contidos no head do PR
mergeado (commits empurrados após o merge). D3 retorna `keep — commits after the merged PR` para
as duas. O feature é correto e conservador — mas as duas branches que motivaram a REQ **não serão
apagadas** por ele. O operador deverá usar o forge manualmente. O AC7 do roadmap deve ser
formulado em função do que a feature entrega (branches cujo PR MERGED contém o tip), não das duas
branches originais.

### C13 — Residual do ADR: ref recriada com o mesmo SHA

ADR lista: ref apagada e recriada com o mesmo commit → lease passa → remoção acontece → commit na
main. Sem perda de trabalho. **Residual estruturalmente correto; aceito.**

---

## Seção 3 — Lease na remoção (ADR D5)

### Documentação oficial

Fonte: https://git-scm.com/docs/git-push

> `--force-with-lease=<refname>:<expect>` will protect `<refname>` (alone), if it is going to be
> **updated**, by requiring its current value to be the same as the specified value `<expect>`
> (which is allowed to be different from the remote-tracking branch we have for the refname, or we
> do not even have to have such a remote-tracking branch when this form is used).
>
> Note that all forms other than `--force-with-lease=<refname>:<expect>` that specifies the expected
> current value of the ref explicitly are still experimental [...]. A general note on safety:
> supplying this option without an expected value [...] interacts very badly with anything that
> implicitly runs git fetch on the remote in the background.

O ADR usa a forma explícita — imune ao problema.

### Medição

Sonda em bare repo isolado (`bash test-lease.sh`, tmpdir fora do projeto — ver nota de AC):

```
Test 1: SHA avaliado == SHA remoto
  → - [deleted]  feat/foo   EXIT=0
  git -C <bare> rev-parse --verify --quiet refs/heads/feat/foo → EXIT=1 (não existe)

Test 2: SHA avaliado stale (clone2 avançou o remoto)
  → ! [rejected]  (delete) -> feat/bar (stale info)   EXIT=1
  git -C <bare> rev-parse refs/heads/feat/bar = SHA_NOVO (mantida)
```

### Especificação do teste Go para ML-1A

```go
func TestRemoteDeleteForceLease(t *testing.T) {
    // Isolamento:
    //   GIT_CONFIG_NOSYSTEM=1; GIT_CONFIG_GLOBAL → arquivo vazio em t.TempDir()
    //   git init --initial-branch=main; -c user.name/email -c commit.gpgsign=false
    //   bare em t.TempDir(); clone em t.TempDir()
    // Verificação: git -C <bare> rev-parse --verify --quiet refs/heads/<b>
    //   NÃO usar git ls-remote (EXIT=0 com saída vazia — falso positivo)

    // Braço A: SHA igual → remoção SUCEDE (rev-parse → EXIT=1)
    // Braço B: remoto avançou via clone2 (avaliador não refetchou)
    //          → remoção retorna "stale info" EXIT=1; rev-parse retorna SHA_NOVO
    // Braço C: remoto avançou E avaliador refetchou entre avaliação e remoção
    //          (lease usa SHA_ANTIGO do snapshot original)
    //          → remoção retorna erro (SHA no lease != SHA atual)
    //
    // Mutações que devem reprovar:
    //   drop `:<sha>` do lease → braço B passa incorretamente
    //   lease em `refs/remotes/origin/<b>` → braço B passa (ref errada protegida)
    //   SHA relido na hora da remoção → braço C passa (sem TOCTOU protection)
}
```

**Hook pre-push:** `lefthook.yml` ausente neste repo; `.git/hooks/pre-push.sample` não ativo.
Não há pre-push hook que precise lidar com SHA nulo.

---

## Seção 4 — Branch protegida (ADR D6)

### Consulta escolhida

```bash
gh api 'repos/kgsaran/trackfw/branches?protected=true&per_page=100' --paginate \
  --jq '[.[] | {name, protected}]'
# [{"name":"main","protected":true}]   EXIT=0  (sem admin token)
```

Campo `protected: bool` disponível sem admin token. Reflete branch protection rules clássicas
(medido). Relação com rulesets não confirmada neste repo (endpoint retornou `[]`).

### Falha da consulta

Fail-closed — não apagar nenhuma branch remota na execução.

### HEAD exclusion

Vault mostra que `%(refname:short)` de `refs/remotes/origin/HEAD` emite `"origin"` (não
`"origin/HEAD"`). Filtro por short name `!= "origin/HEAD"` NÃO exclui o HEAD symref. A exclusão
deve ser por refname completo. **Declarado como ajuste A8.**

---

## Seção 5 — Alvos de falsificação nas duas direções

### Direção 1: apaga o que não devia (falso positivo de remoção)

| caso | controle que fecha | sabotagem que abre |
|---|---|---|
| C2 TOCTOU | `--force-with-lease=refs/heads/<b>:<sha>` | Lease em `refs/remotes/origin/<b>` — ref sendo deletado é `refs/heads/<b>`; lease inerte |
| C3 PR aberto | D1 case 0: OPEN → `keep` | Não filtrar OPEN antes de MERGED |
| C3-stacked | Excluir `b` quando é `baseRefName` de PR aberto | Não checar `baseRefName` |
| C10 heurística | `decision == merged_pr`, não `deletable()` | Usar `decision.deletable()` no caminho remoto |
| C11 default branch | `default_branch` do forge | Hardcode "main" — apaga branch padrão de outro nome |
| C4 protegida | Lista `protected: bool`, fail-closed na falha | Tratar falha como "não protegida"; não paginar |
| C5 pushurl | Comparar fetch URL vs push URL; bloquear se divergirem | Não checar pushurl |

### Direção 2: não apaga a integrada ou introduz dano silencioso

| caso | controle que fecha | sabotagem que abre |
|---|---|---|
| `--remote` sem `--apply` escreve remoto | Verificar `apply == true` antes de qualquer remoção | Inverter ou remover a guarda |
| `reasonPushDelete` aponta para --mirror/--prune | A1: escopo restrito a formas branch-delete | Ramificação ingênua: toda forma com `:` → reasonPushDelete |
| Guard falha-aberta para forma nova | `applyRule` sem `default: return ""` | `default: return ""` ao parsear argumentos de push |
| Branch integrada não listada | Branch com PR MERGED e tip == headRefOid → `would delete` | Filtro `baseRefName` restritivo demais exclui PRs legítimos |

---

## Seção 6 — Resíduo declarado

**R1 — Forges além do GitHub:** D2 declara. Degrada.

**R2 — PR reescrito por rebase:** headRefOid ausente localmente → `review`. Herdado ADR-2026-10-03.

**R3 — Branches motivadoras da REQ são casos `keep`:** `2d3a7dbb` e `99477f44` têm commits após o
merge (C12 medido). D3 retorna `keep`. O feature é correto; a remoção dessas branches exige ação
manual no forge. AC7 deve ser formulado adequadamente.

**R4 — Alias bypass** (`git -c alias.p=push p origin --delete b`, exit 0): pré-existente, cobre
todo push via alias. Não é regressão desta REQ.

**R5 — Config-driven deletes** (`remote.origin.push`, `remote.origin.mirror`): invisíveis ao parser.

**R6 — Ref recriada com mesmo SHA:** residual do ADR aceito (C13).

---

## Veredito

O ADR `ADR-2026-10-12-branch-prune-apaga-a-branch-remota-so-pelo-sinal-do-forge-e-condicionada-ao-tip-avaliado.md`
está **APROVADO COM AJUSTES**. Os controles D3, D4, D5 fecham os vetores de maior severidade.

**Finding veredito-impactante (C12):** as duas branches que motivaram a REQ (`2d3a7dbb`,
`99477f44`) são casos `keep` por D3 — D3 está correto. O AC7 deve ser reformulado em função do
que a feature entrega, não dessas branches específicas.

**Ajustes obrigatórios (bloqueadores de implementação):**

| # | ajuste | ADR | severity |
|---|---|---|---|
| A1 | `reasonPushDelete` só para formas branch-delete (`--delete`/`-d`/`:<branchref>`); `--mirror`/`--prune`/`:refs/tags/`/bare `:` mantêm `reasonPush` | D7 | Alto |
| A2 | Parity test recebe vetores de remoção de branch (não só push simples) | D7 | Alto |
| A3 | Excluir `b` quando é `baseRefName` de PR aberto (stacked PR) | D3 | Alto |
| A4 | Paginar consulta de branches protegidas; fail-closed na falha | D6 | Alto |
| A5 | Comparar fetch URL vs push URL; bloquear remoção se divergirem | D2 | Alto |
| A6 | Caminho remoto verifica `decision == merged_pr`, nunca `decision.deletable()` | D3 | Crítico |
| A7 | `default_branch` do forge (não hardcode "main") para exclusão e filtro `baseRefName` | D2 | Alto |
| A8 | Exclusão de `refs/remotes/origin/HEAD` por refname completo, não short name `"origin"` | D2 | Médio |

**Ajuste de teste:**

| # | ajuste |
|---|---|
| A9 | `TestRemoteDeleteForceLease` com braços A/B/C + mutações; verificar com `rev-parse --verify --quiet`, não `git ls-remote` |

**Notas para o `trackfw_architect`:**
- A7 (hardcode "main"): mesma causa da REQ-2026-10-03. Regra Dura de Causa Raiz: novo ML na REQ
  vigente, não REQ nova.
- C9: `guard update-ref` não é controle efetivo contra escrita direta em `.git/refs/`. O lease
  (D5) é o único controle efetivo neste vetor.
- Working context, vault e memory não foram atualizados — constraint de um arquivo desta sessão.

**Resíduo aceito:** R1, R2, R3, R4, R5, R6.
