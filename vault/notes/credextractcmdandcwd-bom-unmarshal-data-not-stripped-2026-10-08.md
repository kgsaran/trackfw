# credExtractCmdAndCwd passa `data` (com BOM) em vez de `stripped` para json.Unmarshal

**Data:** 2026-10-08
**REQ:** REQ-2026-10-06
**Função:** `credExtractCmdAndCwd` em `internal/guard/credential.go` linha ~373

## Causa raiz

`credExtractCmdAndCwd` computa `stripped = bytes.TrimPrefix(data, BOM)` e usa `stripped`
para verificar `trimmed[0] == '{'`. No entanto, passa `data` (com BOM intacto) para
`json.Unmarshal`. A stdlib de Go rejeita BOM como JSON inválido (não é parte do RFC 8259),
então `json.Unmarshal` retorna erro → `isJSON=false` → a função retorna `("", "", false)`.

```go
// COMO ESTÁ (bugado):
stripped := bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
trimmed := bytes.TrimLeft(stripped, " \t\r\n")
if len(trimmed) == 0 || trimmed[0] != '{' {
    return "", "", false
}
var root map[string]json.RawMessage
if err := json.Unmarshal(data, &root); err != nil {  // BUG: usa `data`, não `stripped`
    return "", "", false
}

// COMO DEVE SER:
if err := json.Unmarshal(stripped, &root); err != nil {
```

## Efeito

`isJSON=false` desativa Layer 2b em `RunCredential` (linha 87) e `RunCredentialGlobal`
(linha 166). Todos os três fixes do ML-1C regridem para o comportamento legado (blanket
exemption) quando o payload vem com BOM:

| Fix do ML-1C | Branch sem BOM | Branch com BOM | Regressão? |
|---|---|---|---|
| B1: `cat file > /dev/null` | RC=2 | RC=0 | sim (volta a main behavior) |
| R1e: `cat "file"` (aspas) | RC=2 | RC=0 | sim |
| EE4: fs_write + JWT + /dev/null | RC=2 | RC=0 | sim |
| `cat file` (sem redirect) | RC=2 | RC=0 | sim — Branch cai abaixo de main (main RC=2) |

## Trigger

PowerShell pode emitir BOM UTF-8 antes de JSON no stdout. Documentado como comentário de
código no mesmo pacote (commit 782f5767, função `ExtractCommand`): "UTF-8 BOM stripped —
PowerShell may emit one". A nova `credExtractCmdAndCwd` do ML-1C não replicou o tratamento.

## Correção

Linha ~373: `json.Unmarshal(data, &root)` → `json.Unmarshal(stripped, &root)`.
Fix de uma linha; sem efeito em payloads sem BOM (TrimPrefix é no-op).
Cobre ambos escopos (linha 87 e 166 compartilham a mesma função).

## Varredura same-cause

Demais chamadas `json.Unmarshal` em `credential.go` (linhas 380, 396, 420, 428) operam sobre
sub-elementos extraídos de `root`, não sobre `data` bruto. Não afetadas.

## Status

Corrigido em ML-2C (2026-10-08). `json.Unmarshal(data, &root)` → `json.Unmarshal(stripped, &root)` em `credExtractCmdAndCwd`. Build limpo, testes verdes.
