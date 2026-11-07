---
name: guard-credencial-comentarios-de-teste-disparam-o-guard-2026-11-06
description: Comentários em arquivos de teste que citam o token completo (ex. PEM header em docstring) são varridos pelo guard tanto quanto o código — a divisão em fragmentos não ajuda quando o literal aparece íntegro num comentário.
metadata:
  type: project
---

## Fato

O censo da Wave 0 (ML-1B, REQ-2026-11-06) encontrou um match de PEM private key em
`internal/guard/credential_patterns_test.go`. Todos os tokens no CÓDIGO já estavam
divididos por concatenação (cabeçalho PEM partido em três fragmentos concatenados). O match
veio de um **comentário de docstring** na linha 209:

```
// PEM note: the PEM header "<cabeçalho PEM de chave privada, omitido de propósito>" contains spaces...
```

A string `<cabeçalho PEM de chave privada, omitido de propósito>` aparecia inteira no comentário e casou com
o regex `-----BEGIN [A-Z ]*PRIVATE KEY-----`.

**Por que:** o guard (`credScanInput`) aplica os regexes sobre os bytes brutos do arquivo.
Ele não distingue código de comentário. Um comentário com o token completo é tão
"visível" quanto um literal no código.

**Why:** O mesmo padrão se aplica a qualquer arquivo de teste que mencione o token num
docstring para explicar o comportamento esperado. A regra de fragmentação (ADR D3,
REQ AC3) se aplica a comentários tanto quanto a valores Go.

## Como aplicar

Ao criar testes para o guard de credencial:
1. Não escrever o token completo em comentários de função — referenciar a entrada do
   `credEphemeralTable` ou usar a forma elíptica: `-----BEGIN...PRIVATE KEY-----`.
2. O teste `TestFixturesSemTokenLiteral` (ML-1B) detecta regressão automaticamente:
   aplica todos os `credPatterns` sobre o arquivo-fonte e reprova se casar.

## Links

- [[feedback_falsificacao_nas_duas_direcoes]]
- REQ-2026-11-06-guard-de-credencial-atribui-ao-payload-o-achado-que-veio-de-arquivo-referenciado
- ADR-2026-11-06-guard-de-credencial-diz-de-onde-veio-o-achado (D3)
