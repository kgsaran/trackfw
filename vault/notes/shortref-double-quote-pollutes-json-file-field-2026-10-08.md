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

## Fix

Em `upstreamSymrefShort()`, após obter o short name:
```go
return strings.ReplaceAll(s, `"`, "")
```

Acrescentar teste `TestReqDoneOpenCriteria_DoubleQuoteBranchName` que:
1. Cria `$fork/.git/refs/remotes/upstream/HEAD → refs/remotes/upstream/main"evil"`
2. Cria `$fork/.git/refs/remotes/upstream/main"evil"` apontando para um commit real
3. Verifica `file == ""` no JSON (não `evil`)
