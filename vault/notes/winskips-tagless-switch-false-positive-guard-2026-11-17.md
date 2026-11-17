# winskips: tagless switch default false positive — guard com hasAnyGOOSCase (2026-11-17)

**Contexto:** ML-FA3 (REQ-2026-11-15), fix do achado 4-A1 do red team (tagless switch invisível ao gate).

## Problema

Ao implementar o suporte a tagless switch (`switch { case runtime.GOOS == "windows": t.Skip() }`),
o handler checava o `default:` case quando `hasCondTrueCase == false`. Isso era correto para switches
onde algum case tinha uma comparação GOOS. Mas para tagless switches SEM nenhuma comparação GOOS
(ex.: `switch { case joined == "foo": ...; default: return "", nil }`), o default era checado de
qualquer forma, e o `return` no default disparava detecção falsa.

**13 falsos positivos** foram gerados em:
- `internal/commands/branch_default_ls_remote_test.go`
- `internal/commands/branch_prune_default_branch_test.go`
- `internal/commands/ship_test.go`
- `internal/validator/validator_guard_wiring_concordance_external_test.go`

Padrão common: tagless switch com lógica de dispatch (cases com `strings.Contains`, `strings.HasPrefix`,
etc.) e default com `t.Fatalf(...)` + `return`, ou default com `continue`.

## Fix

Adicionado `hasAnyGOOSCase bool` no loop das cases. O default só é checado quando:
1. Pelo menos um case tinha uma comparação GOOS (`hasAnyGOOSCase == true`), E
2. Nenhuma case foi `condTrue` no Windows (`hasCondTrueCase == false`)

```go
if hasAnyGOOSCase && !hasCondTrueCase {
    // check default
}
```

## Por que não foi óbvio

O advisor indicou "check default only if no condTrue case was found" — isso está correto para
switches GOOS-relacionados. A condição implícita (que o switch seja GOOS-relacionado) não estava
documentada e ficou faltando na primeira implementação.

## Referência

`internal/winskips/winskips.go`, função `nodeHasPlatformExit`, ramo `stmt.Tag == nil`,
comentário "Guard: if NO case in the switch contains a GOOS comparison at all..."
