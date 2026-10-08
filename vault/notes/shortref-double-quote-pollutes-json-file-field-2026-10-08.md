# shortRef não sanitizado: `"` no nome do ramo polui `warnings[].file` no JSON

**Data:** 2026-10-08
**ML:** ML-2A (red-team, REQ-2026-10-08)
**Arquivo:** `internal/validator/validator_req_done_criteria.go` — `upstreamSymrefShort()`

## Problema

`git check-ref-format 'refs/heads/main"evil"'` retorna exit=0 — aspas duplas são válidas em
nomes de ramo. O resultado de `git symbolic-ref --short refs/remotes/upstream/HEAD` é inserido
diretamente no format string `"(%d inherited from %s)"` sem sanitização.

`extractFile` (result.go:40) usa regex `"([^"]+)"` para extrair o campo `file` do JSON. Se o
nome do ramo for `main"evil"`, o campo `file` do warning `req_done_open_criteria` vira `evil`
em vez de `""`.

## Impacto

- JSON `warnings[].file`: path falso extraído do nome do ramo (MEDIUM).
- Texto, message, enforcement, exit code: inalterados.
- Violação da restrição §6.2 da Wave 0 para esta REQ: "Nenhuma aspa dupla na parentética".
- Só reachable via fallback HEAD (step 4 de `upstreamInheritedInfo`): upstream/main e
  upstream/master ausentes. Caminho normal não é afetado.

## Fix (implementado em ML-2B)

Allowlist em `upstreamInheritedInfo` (step 4), não remoção pontual de `"`.
Em `validator_req_done_criteria.go`:

```go
var shortRefSafeRe = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

func shortRefSafe(s string) bool {
    if s == "" || strings.HasPrefix(s, "-") {
        return false
    }
    return shortRefSafeRe.MatchString(s)
}
```

No step 4 de `upstreamInheritedInfo`, após `upstreamSymrefShort`:
```go
if !shortRefSafe(shortRef) {
    return "(upstream tried main, master: ref unresolvable)", 0
}
```

Testes acrescentados:
- `TestReqDoneOpenCriteria_DoubleQuoteBranchName`: cria ref files diretamente, verifica
  parentética = unresolvable e `RuleItem.File == ""` no JSON.
- `TestReqDoneOpenCriteria_SpecialBranchNamePassesAllowlist`: direção oposta —
  `feature/special-branch` continua aparecendo via step 4 (HEAD fallback).
