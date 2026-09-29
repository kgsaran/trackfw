---
name: windows-known-failures-14a-entrada-ausente-das-tabelas-wave0-2026-09-29
description: Wave 0 triou 13 entradas em 4 grupos (4+4+2+3); TestUpdateMigratesKnownCodexAndPreservesUnknown é a 14ª, ausente das tabelas — causa inferida de source.
metadata:
  type: project
---

## Fato

A Wave 0 de 2026-09-29 (`docs/seguranca/2026-09-29-wave0-ratchet.md`) triou as 14 entradas de
`.github/windows-known-failures.json` em quatro grupos:

| Grupo | Entradas |
|---|---|
| A — `os.Chmod` silenciosa no NTFS | 4 |
| B — CRLF renderer | 4 |
| C — comando externo (symlink privil. + fork-bomb) | 2 |
| D — representação de caminho | 3 |

**Soma: 13.** A 14ª entrada — `TestUpdateMigratesKnownCodexAndPreservesUnknown` — não aparece em
nenhuma tabela da Wave 0.

**Why:** a triagem da Wave 0 foi feita sobre o log do CI `windows-full-suites` (run 34478752778).
A 14ª entrada provavelmente não falhou nessa execução específica ou ficou fora do escopo de triagem
das 4 classes identificadas.

## Causa inferida (marcada INFER no JSON)

Lido de `internal/generators/update_test.go:145`:

```go
if !strings.Contains(string(manifest), backendPath) {
```

onde `backendPath` vem de `filepath.Join(...)` — que no Windows usa `\`. O manifest pode armazenar
caminhos com `/` normalizado. `strings.Contains` falha quando as duas formas divergem.

Classificado como **Grupo D: representação de caminho** — mesma família de
`TestThirdPartyInstall_*` (8.3 short-name) mas mecanismo diferente (separador, não expansão).

## How to apply

Ao auditar `.github/windows-known-failures.json` ou gerar novo `reason` para entradas do Grupo D:
- As duas entradas `TestThirdPartyInstall_*` são 8.3 short-name (`RUNNER~1` em `filepath.Rel`).
- `TestUpdateMigratesKnownCodexAndPreservesUnknown` é separador (`\` vs `/` em `strings.Contains`).
- São mecanismos distintos — não consolidar as razões.

Ao contar grupos na Wave 0: o Grupo D tem **4 entradas** no JSON, mas **3 na tabela** da Wave 0.
A 4ª (`TestUpdateMigratesKnownCodexAndPreservesUnknown`) é inferida e não foi medida no log do CI.
Se uma execução futura fornecer a mensagem de falha real, substituir `INFER` por `MEASURED`.
