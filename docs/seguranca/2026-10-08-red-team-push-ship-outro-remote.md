# Red-Team ML-2A: push/ship avisam sobre branch de outro remote

> Data: 2026-10-08 | Agente: hades-tf | ML-2A
> Commit auditado: `4c8ade8e` (fix/push-e-ship-avisam-sobre-branch-de-outro-remote)
> Wave 0: docs/seguranca/2026-10-08-wave0-push-ship-outro-remote.md
> REQ: docs/req/REQ-2026-10-08-push-e-ship-avisam-sobre-branch-de-outro-remote-como-trabalho-nao-mesclado.md

---

## Metodologia

Leitura integral do diff `git show HEAD -- internal/commands/ship.go` e `ship_test.go`. Execução de cada caso de ataque via script Go (`edge_cases_main.go` em scratchpad). Execução de `go test -run TestDetectPendingSquashMerges_Issue547_MultiRemote` em baseline + duas mutações. Nenhum arquivo de produto modificado.

---

## Achados

### A1 — Descarte de HEAD: `strings.Cut(refName, "/")` + `after == "HEAD"` (sem severidade — correto)

**Vetor atacado:** os seis casos do handoff para o filtro HEAD.

**Análise linha a linha (ship.go:810–814):**

```go
refName, _, _ := strings.Cut(candidate, " -> ")
if _, after, ok := strings.Cut(refName, "/"); ok && after == "HEAD" {
    continue
}
```

| Entrada (pós-TrimSpace) | refName | after (1º Cut em "/") | Descartado? | Esperado |
|---|---|---|---|---|
| `origin/HEAD -> origin/main` | `origin/HEAD` | `HEAD` | sim | sim (pointer) |
| `origin/HEAD` (bare) | `origin/HEAD` | `HEAD` | sim | sim (pointer) |
| `origin/fix/HEAD` | `origin/fix/HEAD` | `fix/HEAD` | não | não (branch real) |
| `origin/HEADER` | `origin/HEADER` | `HEADER` | não | não (branch real) |
| `origin/fix/HEADER-parse` | `origin/fix/HEADER-parse` | `fix/HEADER-parse` | não | não (branch real) |
| `team/upstream/HEAD` | `team/upstream/HEAD` | `upstream/HEAD` | não | não (mas filtrado por origin prefix) |
| `origin2/HEAD -> origin2/main` | `origin2/HEAD` | `HEAD` | sim | sim (pointer de outro remote) |
| `  origin/feat\r` | `origin/feat` (após TrimSpace) | `feat` | não | não (branch real) |

**Observação sobre `team/upstream/HEAD`:** o filtro HEAD usa o primeiro `/`, portanto `team/upstream/HEAD` produz `after = "upstream/HEAD"`, que não casa com `"HEAD"` — a linha passa pelo filtro HEAD. Ela é então descartada pelo filtro `HasPrefix("origin/")`. O resultado final é correto. Não é um bug — os dois filtros são complementares e a composição é segura.

**Resultado:** nenhum defeito. Todos os casos produzem o comportamento correto.

---

### A2 — HasPrefix("origin/"): remotes `origin-mirror`, `originx`, branches locais, output localizado (sem severidade — correto)

**Vetor atacado:** se o recorte `HasPrefix("origin/")` esconde trabalho do usuário ou filtra incorretamente.

| Entrada | HasPrefix("origin/") | Ação | Correto? |
|---|---|---|---|
| `origin/feat/pending` | true | shortName = `feat/pending` | sim |
| `origin-mirror/feat` | false | skip | sim |
| `originx/feat` | false | skip | sim |
| branch local (ex: `feat/local`) | não aparece em `git branch -r` | n/a | sim |

**Branches locais:** `git branch -r` lista apenas remote-tracking refs. Uma branch local nomeada `origin-foo` ou similar não aparece nesse output. O filtro não afeta branches locais.

**Output localizado:** o git não localiza nomes de branches; apenas algumas mensagens de status. O formato de `git branch -r` (`  remote/branch`) é constante entre locales. Sem risco.

**Argumento de fechamento (Wave 0 §3.1 confirmado):** `tf push` e `tf ship` sempre empurram para `origin`. Portanto filtrar por `HasPrefix("origin/")` captura 100% das branches gerenciadas pelo trackfw. Branches de outros remotes não são candidatas a aviso.

**Resultado:** nenhum defeito. O filtro não esconde nenhum trabalho próprio.

---

### A3 — Outros callers e sítios com mesmo padrão (enumeração reconfirmada)

**Grep executado:**

```
grep -n "detectPendingSquashMerges" internal/commands/*.go
```

Callers do produto (excluindo test/comment):

| Arquivo | Linha | Ponto |
|---|---|---|
| `push.go` | 241 | `detectPendingSquashMerges(branch, deps.execGit, deps.ghExec, deps.out)` |
| `ship.go` | 395 | `detectPendingSquashMerges(branch, deps.execGit, deps.ghExec, deps.out)` |

Nenhum outro caller. A função existe em um único sítio (`ship.go:794`). O fix nesse sítio cobre ambos os callers — confirmado pelo Wave 0 §1 e reconfirmado agora.

Sítios com `"branch", "-r"` ou `TrimPrefix.*"origin/"`: apenas o sítio corrigido (grep devolveu os mesmos hits do Wave 0).

**Resultado:** enumeração fechada confirmada. Nenhum sítio adicional.

---

### A4 — Mutações: testes falham corretamente

**Baseline:**
```
go test ./internal/commands/ -run TestDetectPendingSquashMerges_Issue547_MultiRemote
PASS (0.135s)
```

**Mutação 1: HasPrefix guard removido**

`if !strings.HasPrefix(candidate, "origin/") { continue }` deletado.

Resultado:
```
FAIL ship_test.go:1932: issue #547 direction 1: must NOT warn for upstream/fix/some-feature
FAIL ship_test.go:1941: issue #547: expected exactly 2 Warning lines; got 3.
```

O stub genérico do teste faz `upstream/fix/some-feature` chegar em `evaluateBranchIntegration` e receber `pending_work` — tornando a assertion de direção 1 não-vacuosa. ✅

**Mutação 2: revertido a `strings.Contains(candidate, "HEAD")`**

Resultado:
```
FAIL ship_test.go:1928: issue #547 R3: must warn for origin/fix/HEADER-parse
FAIL ship_test.go:1941: issue #547: expected exactly 2 Warning lines; got 1.
```

`origin/fix/HEADER-parse` silenciada por `Contains`. ✅

**Restauração verificada:** baseline PASS após restaurar ship.go.

---

## Residuais pré-existentes (não alterados por este commit)

**R2 (Wave 0):** remote principal não chamado `origin` — intencional, fora do escopo.

**R3 (Wave 0):** o próprio commit corrigiu o `Contains("HEAD")` → agora `origin/fix/HEADER-parse` é avaliado corretamente. R3 está **fechado** por este commit; o commit message registra isso.

**R4 (Wave 0):** falso positivo estrutural de squash-merge quando `origin/main` local está desatualizado. Não afetado por este commit.

**Residual novo desta revisão:** nenhum.

---

## Veredito

**APROVA.**

O fix é correto e completo para o escopo da REQ. Os dois mecanismos corrigidos (`HasPrefix("origin/")` e predicado HEAD exato) cobrem todos os vetores atacados. O teste `TestDetectPendingSquashMerges_Issue547_MultiRemote` falha corretamente com ambas as mutações solicitadas (HasPrefix removido; Contains revertido). Nenhum sítio adicional de mesma causa foi encontrado. Nenhum achado de severidade.
