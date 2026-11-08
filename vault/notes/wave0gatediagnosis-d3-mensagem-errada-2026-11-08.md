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

## Ação necessária

Novo ML na REQ de idioma (ou nova REQ se a causa for diferente):
- Adicionar `Wave0GateDuplicate` ao tipo de diagnóstico em `internal/roadmapdoc/`
- Atualizar `Wave0GateDiagnosis` para retornar `Wave0GateDuplicate` quando o erro de
  `ParseGates` contiver `"duplicate gates header"`
- Atualizar `internal/validator/validator_roadmap_gates.go` para usar a mensagem correta
  por variante de diagnóstico
