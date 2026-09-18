# `%(refname:short)` para `refs/remotes/origin/HEAD` emite `"origin"`, não `"origin/HEAD"`

> Descoberto em ML-2A do issue #387 (2026-09-17). Ação 0 da REQ.

## Fato

`git for-each-ref --format='%(refname:short)' refs/remotes/origin/` emite **`"origin"`** (não
`"origin/HEAD"`) para o symbolic ref `refs/remotes/origin/HEAD`.

Medição direta (macOS, git 2.x):

```
$ git symbolic-ref refs/remotes/origin/HEAD refs/remotes/origin/trunk
$ git for-each-ref --format='%(refname:short)' refs/remotes/origin/
origin          ← refs/remotes/origin/HEAD
origin/trunk    ← refs/remotes/origin/trunk
```

## Por que isso importa

`deriveOriginDefaultBranch()` em `validator_credential_guard_integrity.go` filtra o HEAD pointer
para não contá-lo como branch real. O filtro original era:

```go
if line == "" || line == "origin/HEAD" {
    continue
}
```

Como `%(refname:short)` emite `"origin"` (não `"origin/HEAD"`), o filtro nunca casava.
Resultado: repos onde `git clone` escreveu `refs/remotes/origin/HEAD` (todos os clones normais)
tinham `"origin"` incluído na lista de branches, quebrando a contagem de branch único (`branches`
ficava com 2 entradas: `["origin", "origin/trunk"]`) e a derivação unambígua falhava (`ok = false`).

## Fix

Filtrar os dois:

```go
if line == "" || line == "origin" || line == "origin/HEAD" {
    continue
}
```

Manter `"origin/HEAD"` como guard extra (custo zero) previne regressão em caso de variação entre
versões de git.

## Teste de referência

`TestDeriveOriginDefaultBranch_OriginHEADIsFiltered` em
`internal/validator/validator_lenient_ml2a_test.go` — prova que `origin/trunk` é derivado
corretamente quando `refs/remotes/origin/HEAD` está presente e há um único branch real.
