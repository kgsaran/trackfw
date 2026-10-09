# Wave 0 — Threat Model: push/ship avisam sobre branch de outro remote

> Data: 2026-10-08 | Agente: hades-tf | ML-0A
> REQ: docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md
> Roadmap: docs/roadmaps/wip/ROADMAP-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md

---

## 1. Enumeração fechada — sítios que listam `branch -r`, assumem `origin/` ou usam `TrimPrefix(..., "origin/")`

### Padrões varridos em `internal/`

| Padrão | Hits (excluindo test/testdata) |
|---|---|
| `"branch", "-r"` | 1 (`ship.go:795`) |
| `--no-merged` | 1 (`ship.go:795`) |
| `TrimPrefix.*"origin/` | 1 (`ship.go:809`) |
| `HasPrefix.*"origin/` | 0 |
| concatenação `"origin/"+` | 4 (`ship.go:815`, `release.go:289`, `release.go:364`, `push.go:257`) |
| `--remotes` | 0 |
| `branch.*-a` / `--all` | 0 |
| `"--merged"` | 0 |
| `"ls-remote"` | 1 (`release.go:310` — tag check, always pinado a `origin`) |
| `refs/remotes/` | `barrier.go` (acesso pontual a `refs/remotes/origin/main`), `ship.go:614`, `validator_credential_guard_integrity.go:336` |

### Sítio defeituoso único

| Arquivo | Linhas | Padrão |
|---|---|---|
| `internal/commands/ship.go` | 795, 809, 815 | `git branch -r --no-merged origin/main`, `TrimPrefix(candidate, "origin/")`, `"origin/"+shortName` |

Função: `detectPendingSquashMerges` (ship.go:794–823). Chamada de dois lugares com a mesma função compartilhada:
- `runShip` (ship.go:395, dentro do bloco `else` pós-fetch bem-sucedido)
- `runPush` (push.go:241, mesmo bloco)

Nota: `--dry-run` não chama `detectPendingSquashMerges`. Em ambos os comandos, a função é invocada somente no ramo `else` após `fetch origin --prune` ter retornado sem erro. Um teste com `--dry-run` nunca exercita o defeito.

### Sítios com `"origin/"` que NÃO têm defeito multi-remote

| Arquivo | Linha(s) | Uso | Defeito multi-remote? |
|---|---|---|---|
| `branch_prune.go:19` | `branchPruneDefaultRemoteRef = "origin/main"` | Avaliação de branches LOCAIS via `git branch --format=%(refname:short)`. A população é local — nenhum ref de remote não-origin é incluído. | Não |
| `release.go:289,364` | `"origin/"+base`, `"origin/"+repoInfo.DefaultBranch` | Resolve commit-target para tag; hardcoded por decisão ADR-2026-08-19. | Não |
| `release.go:310` | `ls-remote --tags origin refs/tags/...` | Verifica existência de tag no remote; sempre `origin` explícito. | Não |
| `validator.go:229–399,783` | `origin/main` como âncora de severidade de regras | Âncora de severidade via `git ls-tree`. Não lista branches. | Não |
| `ship.go:614,625` | `gitSymbolicRefOriginHeadPrefix = "refs/remotes/origin/"` | Resolução de `defaultBaseBranch` para corpo do PR (um único `symbolic-ref`). | Não |
| `barrier.go:423,434` | `refs/remotes/origin/main^{commit}`, `refs/remotes/origin/main:<path>` | Verificação de presença de roadmap em origin/main para o gate de barreira. | Não |
| `push.go`, `ship.go` | `remote get-url origin`, `fetch origin --prune`, `push -u origin` | Comandos de escrita/fetch — sempre intentionally `origin`. | Não |

**Enumeração fechada.** Nenhum sítio em `internal/` usa `git branch -r` com ou sem `TrimPrefix(..., "origin/")` exceto o sítio defeituoso único.

---

## 2. Medição — repositório temporário com dois remotes

### Setup do fixture

Repositório construído em `$SCRATCH/two-remote-test3/work/` usando `git commit-tree` + `git update-ref` (sem `git commit` nem `git push`). Dois bare repos locais:

- `bare-origin`: `main` (MAIN_SHA, `README.md`) e `feat/pending` (PENDING_SHA, adiciona `feature.md`)
- `bare-upstream`: `main` (= MAIN_SHA, já integrado) e `fix/some-feature` (UPSTREAM_SHA, adiciona `upstream.md`)

Binário: compilado do HEAD desta branch (`fix/push-e-ship-avisam-sobre-branch-de-outro-remote`) via `go build -o $SCRATCH/tf ./cmd/trackfw`.

Verificação de equivalência: `git diff main..HEAD --stat -- internal/ cmd/` retornou vazio — branch de trabalho idêntica a `main` para o produto.

### Saída de `git branch -r --no-merged origin/main` (input da função)

```
  origin/feat/pending
  upstream/fix/some-feature
```

`upstream/main` não aparece: aponta para o mesmo commit que `origin/main` — controle negativo confirmado.

### Caminho do forge (`queryForgePRs`)

Na fixture, `origin` aponta para `file:///…/bare-origin`. `queryForgePRs` chama `git remote get-url origin` → URL local → `parseHostOwnerRepo` falha (não é github.com) → D2 (degradação) → `snapshot = nil`. Confirmado pelo output de `tf branch prune` no mesmo fixture: `"Note: forge PR signal not available — using content heuristic only. Cause: could not parse origin remote URL..."`.

No caminho D1 (forge ativo, github.com URL, `gh` disponível): `evaluateBranchWithForge` busca PR cujo `headRefName == prName`. Para `prName = "upstream/fix/some-feature"`, nenhum PR em `origin` corresponderia (PRs de origin têm `headRefName = "fix/..."`, não `"upstream/fix/..."`). A função cai no case 5 → content heuristic → mesmo resultado: `pending_work`. O defeito existe em ambos os caminhos (D1 e D2).

### Saída de `tf push` — defeito reproduzido

```
Branch: chore/test-push
Governance: skipped (chore/docs branch)
Warning: branch "feat/pending" appears to have unmerged changes vs origin/main.
Warning: branch "upstream/fix/some-feature" appears to have unmerged changes vs origin/main.
Pushed: chore/test-push → origin/chore/test-push
```

- `feat/pending`: aviso correto — branch de `origin` com trabalho pendente real.
- `upstream/fix/some-feature`: **aviso falso** — branch de `upstream`, não de `origin`.
- `upstream/main`: silencioso — controle negativo confirmado.

### Prova do mecanismo linha a linha

```
ship.go:795  gitExec("branch", "-r", "--no-merged", "origin/main")
             → "origin/feat/pending\nupstream/fix/some-feature"

ship.go:803  candidate = "upstream/fix/some-feature"
ship.go:805  strings.Contains("upstream/fix/some-feature", "HEAD") = false → não filtrado
ship.go:809  shortName = TrimPrefix("upstream/fix/some-feature", "origin/")
                       = "upstream/fix/some-feature"  ← prefixo não casou, inalterado

ship.go:815  eval = evaluateBranchWithForge(
                 ref       = "upstream/fix/some-feature",
                 prName    = "upstream/fix/some-feature",
                 upstream  = "origin/upstream/fix/some-feature",  ← não existe
                 snapshot  = nil (D2),
                 gitExec)

             evaluateBranchIntegration:
               merge-base(origin/main, upstream/fix/some-feature) = MAIN_SHA
               diff MAIN_SHA..upstream/fix/some-feature = upstream.md (non-empty → touched)
               diff origin/main..upstream/fix/some-feature -- upstream.md = upstream.md (non-empty → diverg)
               → Decision: pending_work

ship.go:821  Warning: branch "upstream/fix/some-feature" appears to have unmerged changes vs origin/main.
             ← aviso falso emitido
```

### `tf branch prune` — sem defeito multi-remote

`branch prune` opera sobre branches LOCAIS (`git branch --format=%(refname:short)`). No fixture, apenas `chore/test-push` (current) e `main` (default) existem localmente. Refs de rastreamento como `upstream/fix/some-feature` não são candidatos. Saída confirmada: `[dry-run] nothing to delete.`

---

## 3. Threat model

### 3.1 — O recorte a `origin/*` pode esconder trabalho nosso?

A correção proposta (REQ saída 1) adiciona `if !strings.HasPrefix(candidate, "origin/") { continue }` antes da avaliação de cada candidato. Branches de `origin` continuam sendo avaliadas exatamente como antes.

Argumento que realmente fecha: `buildPushArgs` sempre empurra para `origin` (`push -u origin <branch>`). O ciclo de entrega governado por `trackfw` só produz branches em `origin`. Portanto, filtrar por `origin/` captura 100% das branches relevantes para este comando.

Cenário extremo: usuário que trabalha exclusivamente em `upstream` e nunca cria branches em `origin`. Nesse caso, `tf push` não funciona de nenhuma forma (empurra para `origin` sempre), então a ausência de aviso é correta.

**Caveat pré-existente (não introduzido por esta REQ):** o filtro atual em ship.go:805 é `strings.Contains(candidate, "HEAD")`. O teste verifica se a string "HEAD" aparece em qualquer posição — incluindo como prefixo de "HEADER". Medido ao vivo no fixture: após adicionar `origin/fix/HEADER-parse` (com `upstream.md` diferente), o aviso NÃO é emitido. `strings.Contains("origin/fix/HEADER-parse", "HEAD") = true`. Um branch legítimo de `origin` com "HEAD" no nome é silenciado. Este é um defeito pré-existente, não causado pela correção do multi-remote, e não é escopo desta REQ — declarado como residual R3.

### 3.2 — E se o remote principal do usuário NÃO se chamar `origin`?

O código chumba `"origin"` em oito lugares de `push`/`ship`/`branch_prune`:

| Localização | Hardcoding |
|---|---|
| `push.go:200` | `remote get-url origin` (force-with-lease gate) |
| `push.go:238` | `fetch origin --prune` |
| `push.go` via `buildPushArgs` | `push -u origin <branch>` |
| `ship.go:352` | `remote get-url origin` (force-with-lease gate) |
| `ship.go:393` | `fetch origin --prune` |
| `ship.go:795` | `branch -r --no-merged origin/main` |
| `ship.go:614` | `gitSymbolicRefOriginHeadPrefix = "refs/remotes/origin/"` |
| `branch_prune.go:19` | `branchPruneDefaultRemoteRef = "origin/main"` |

A REQ declara explicitamente no campo "Escopo negativo": "não deriva o remote de `@{u}` (saída 2 da issue)." O comportamento "origin sempre" é uma decisão de produto. Declarado como residual R2.

### 3.3 — Cobertura de testes do caminho multi-remote

Confirmado por grep: zero ocorrências de `upstream/` em todos os `_test.go` de `internal/commands/`. Todos os stubs de `ship_test.go` e `push_test.go` retornam apenas `origin/*` para `branch -r --no-merged`. Mesma família de "fixture não representa o layout do consumidor" (issue #507).

---

## 4. Alvos de falsificação nas duas direções

### Direção 1 — Falso negativo (correção não fecha o defeito)

**Alvo:** após a correção, `tf push` ainda emite aviso para `upstream/fix/some-feature`.

**Onde a sabotagem entra:** se `!strings.HasPrefix(candidate, "origin/")` for adicionado com lógica invertida, ou após o `TrimPrefix` em vez de antes, o defeito persiste.

**Fixture:** stub retorna `"  origin/feat/Y\n  upstream/fix/X\n"` para `branch -r --no-merged origin/main`.

**Assert:** `"upstream/fix/X"` NÃO aparece na saída do comando.

**Mutação que deve vermelho:** deletar a linha `if !strings.HasPrefix(candidate, "origin/") { continue }`.

**Por que não usar `--list 'origin/*'`:** a opção filtra dentro do processo `git` antes de retornar ao Go. Os stubs de teste stubam `execGit` por chave de argv exata — um stub construído para retornar `upstream/*` ignoraria o flag `--list`, tornando o teste vacuoso (o stub retorna o que quer). A correção em Go (`HasPrefix`) é testável com stubs: o filtro está no código Go e é exercitado independentemente do retorno do stub.

### Direção 2 — Falso positivo (correção suprime aviso correto)

**Alvo:** após a correção, `tf push` NÃO emite aviso para `origin/feat/Y` com trabalho pendente.

**Fixture:** mesma acima (stub retorna `origin/feat/Y` e `upstream/fix/X`).

**Assert:** `"feat/Y"` AINDA aparece no aviso (o `TrimPrefix` é verdadeiro por construção após o filtro).

**Mutação que deve vermelho:** trocar `!strings.HasPrefix` por `strings.HasPrefix` (filtrar o oposto — silenciar `origin/*`).

### Controle negativo (deve continuar silencioso)

`upstream/main` não aparece em `git branch -r --no-merged origin/main` — excluído pelo próprio git, independente do filtro Go. Nenhuma lógica adicional é necessária.

---

## 5. Residual declarado

**R1 — `queryForgePRs`/D2 não pior que D1 para branches upstream.** Em ambos os caminhos (forge ativo ou degradado), branches de `upstream/*` acabam no content heuristic com `pending_work`. O defeito existe nas duas rotas.

**R2 — Remote não chamado `origin`.** Usuários cujo remote principal tem outro nome não são cobertos por `tf push`/`tf ship`/`tf branch prune`. Intencional (saída 2 da issue, fora do escopo da REQ).

**R3 — `strings.Contains(candidate, "HEAD")` é mais amplo que necessário.** Qualquer branch com "HEAD" em qualquer posição do nome (ex.: `origin/fix/HEADER-parse`) é silenciada silenciosamente. Medido ao vivo. O bug pré-existe a esta REQ e a correção do multi-remote não o agrava. Requer issue/REQ própria.

**R4 — Squash-merge falso positivo estrutural.** `evaluateBranchIntegration` usa heurística de conteúdo que pode emitir avisos para branches squash-mergeadas onde `origin/main` local não está atualizado. Comportamento documentado em `branch_prune.go:118–122`. Não é escopo desta REQ.

---

## Veredito

**Defeito confirmado e medido ao vivo.** Sítio único: `detectPendingSquashMerges` em `ship.go:795–821`. `git branch -r --no-merged origin/main` retorna refs de todos os remotes; `TrimPrefix(candidate, "origin/")` não filtra refs de outros remotes; a avaliação os trata como trabalho pendente em `origin`.

**Especificação para ML-1A:**

Arquivo: `internal/commands/ship.go`, função `detectPendingSquashMerges`, após o filtro de HEAD (linha ~806), antes de qualquer uso de `TrimPrefix`:

```go
// Skip candidates from remotes other than origin.
// TrimPrefix("origin/", ...) is then a true strip by construction.
// AC2, issue #547.
if !strings.HasPrefix(candidate, "origin/") {
    continue
}
```

Não usar `--list 'origin/*'` como alternativa: stubs de teste ignoram flags do git, tornando o teste vacuoso.

**Fixture de teste obrigatória:** stub `execGit` que retorna `"  origin/feat/pending\n  upstream/fix/some-feature\n"` para `"branch -r --no-merged origin/main"`. Três vetores:
1. `upstream/fix/some-feature` → deve: sem aviso (direção 1; mutação sem `HasPrefix` vira vermelho)
2. `origin/feat/pending` → deve: aviso para `"feat/pending"` (direção 2; `TrimPrefix` verdadeiro por construção)
3. `upstream/main` → não aparece no input do stub (controle negativo)

**Frase de reconciliação** (Regra Dura de Reconciliação, CLAUDE.md): "O teste de direção 1 afirma que candidatos de remotes não-`origin` não disparam aviso após a inserção do filtro `HasPrefix`. O teste de direção 2 afirma que candidatos de `origin` com trabalho pendente continuam disparando aviso, com o `TrimPrefix` produzindo o shortName correto."

**Sítio a corrigir:** um (`ship.go:detectPendingSquashMerges`). Chamado por `runShip` e `runPush` — ambos corrigidos pelo mesmo ponto.

**AC1 atendido:** enumeração fechada por grep (8 padrões), tabela com decisão por sítio, medição ao vivo nas duas direções com dois remotes.
