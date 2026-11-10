# Wave0GateDiagnosis mapeia D3 para Wave0GateMalformed — mensagem errada no validate

**Arquivo:** `internal/validator/validator_roadmap_gates.go:119-123`
**Descoberto em:** ML-1A, REQ de idioma (ADR-2026-11-08), 2026-11-08

## O problema

`Wave0GateDiagnosis` mapeia **todo** erro de `ParseGates` para `Wave0GateMalformed`.
O validator produz a mensagem hardcoded:

```
roadmap %q (wip) Wave 0 gate block was not found: '**Gates da wave:**' is not followed by
a ```bash fence before the next heading (AC7, ADR-2026-09-18 decision 4)
```

Mas quando o erro é D3 (dois cabeçalhos de gates na mesma wave), o erro real de
`ParseGatesLines` é:

```
duplicate gates header: lines N and M both declare a gates block outside fences and
comments — use exactly one gates header per wave (ADR D3)
```

O validator esmaga essa informação e emite a mensagem de "fence ausente", que é errada e não
guia o usuário para a correção certa.

## Escopo

- `Wave0GateDiagnosis` retorna apenas três valores: `Wave0GateOK`, `Wave0GateAbsent`,
  `Wave0GateMalformed`. Não há `Wave0GateDuplicate`.
- O fix mínimo é adicionar `Wave0GateDuplicate` e propagar o erro original para o validator.
- Alternativa mais simples: propagar o `error` de `ParseGates` ao diagnóstico e deixar o
  validator usar `err.Error()` diretamente quando o diagnóstico não for `Absent`.

## Por que não foi corrigido em ML-1A

`internal/validator/` não estava na lista fechada de arquivos do ML-1A. O ML-1A implementa
o leitor bilíngue; a mensagem do validator é um sítio dependente que requer ML próprio.

## Corrigido em ML-1C (REQ de idioma, ADR-2026-11-08)

### Mecanismo da correção

1. **`Wave0GateDuplicate` adicionado** ao `Wave0GateCause` enum em `internal/roadmapdoc/roadmapdoc.go`
   (após `Wave0GateMalformed`, sem reordenar o iota).

2. **`Wave0GateDiagnosisDetail`** (nova função exportada) retorna `(Wave0GateCause, string)`.
   O segundo campo carrega a mensagem original do parser para os casos Duplicate e Malformed.
   `Wave0GateDiagnosis` passa a ser um wrapper de uma linha que descarta o detalhe.

3. **Classificação D3**: `strings.HasPrefix(err.Error(), "duplicate gates header:")` distingue
   o erro de duplicata dos demais erros de `ParseGates`. Continua gateado por `hasAnyNonPendingML`.

4. **`validator_roadmap_gates.go`** passa a chamar `Wave0GateDiagnosisDetail` e adiciona:
   - `case Wave0GateDuplicate`: emite a mensagem do parser verbatim (linha N e M já incluídos).
   - Mensagens de `Wave0GateAbsent` e `Wave0GateMalformed` atualizadas para citar ambas as formas
     (`'**Wave gates:**'` e `'**Gates da wave:**'`).

### Resíduo conhecido (mesmo mecanismo — reportar ao arquiteto)

O caso de "unterminated gates fence" e o FenceMaskCheck fail-closed continuam caindo em
`Wave0GateMalformed` com a mensagem de "not followed by a fence" mesmo que o problema seja
outro. Mesmo mecanismo (parser detail esmaga em mensagem fixa), escopo fora do ML-1C.

### Comportamento do barrier para D3

O barrier já surfaceava a mensagem D3 do parser corretamente via `usageExit` (exit 2),
sem alteração necessária em barrier.go. Verificado empiricamente:
```
trackfw barrier: duplicate gates header: lines 21 and 26 both declare a gates block
outside fences and comments — use exactly one gates header per wave (ADR D3)
```

### Arquivos alterados

- `internal/roadmapdoc/roadmapdoc.go` — `Wave0GateDuplicate` + `Wave0GateDiagnosisDetail`
- `internal/validator/validator_roadmap_gates.go` — novo case + mensagens bilíngues
- `internal/roadmapdoc/wave0_gate_cause_test.go` — novo braço D3 em `TestWave0GateDiagnosis_NamesTheCause`
- `internal/validator/validator_roadmap_gates_ml1c_test.go` — três testes novos
