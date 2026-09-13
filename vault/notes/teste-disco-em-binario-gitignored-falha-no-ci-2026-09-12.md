---
title: teste de existência de arquivo em disco falha no CI quando o arquivo é gitignored por design
tags: [ci, teste, gitignore, skip, ambiente, paridade]
date: 2026-09-12
related: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]]
---

## O padrão

`npm/tests/shim_packaging.test.js` braço "Arm B (disco)" verificava via `fs.existsSync` que o
binário compilado existia em `prototype/packages/trackfw-<plat>/bin/trackfw[.exe]`.

**O binário é gitignored por design** — `prototype/.gitignore` exclui `packages/*/bin/trackfw` e
`packages/*/bin/trackfw.exe`. Ele só existe após `go build` local. O CI nunca o tem.

Resultado: `assert.ok(fs.existsSync(...))` → `AssertionError` no CI. Três checks cascatearam:
`node` → `parity-falsify-shard` (pulado por artefatos ausentes) → `parity` com
`check-falsify-shard-coverage: diretorio de artefatos ausente` → `windows-full-suites`.

## Por que acontece aqui e não nos outros braços

| Braço | Depende de arquivo gitignored? | Funciona no CI? |
|---|---|---|
| A — sem campo `bin` | Não (lê `package.json`) | Sempre ✅ |
| B estático — `files` contém subcaminho | Não (lê `package.json`) | Sempre ✅ |
| B disco — binário existe no disco | **Sim** | Só após build ❌ |
| B runtime — shim resolve sem `bin` | Não (cria fake binary em tmpdir) | Sempre ✅ |

## A correção

Aceitar `t` (TestContext) no teste de disco. Antes de `assert.ok`, verificar existência:

```javascript
test('Arm B (disco) — binário existe no pacote da plataforma atual', (t) => {
  // ...
  if (!fs.existsSync(binPath)) {
    t.skip(
      `binário ausente: ... — gitignored por design; só existe após build local. ` +
      `Ambiente incompleto, não defeito de produto.`
    )
    return
  }
  assert.ok(fs.existsSync(binPath), ...)
})
```

**Verificado nos dois estados:**
- SEM binário (CI): 3 pass, 1 skip nomeado, 0 fail. EXIT 0.
- COM binário (local pós-build): 4 pass, 0 skip, 0 fail. EXIT 0. Braço verifica de verdade.

## Regra geral

**Um teste que verifica existência de arquivo em disco falha no CI se o arquivo for gitignored.**
Isso inclui: binários compilados, artefatos de build, caches gerados, outputs de script.

O padrão correto é: verificar existência antes do assert. Se ausente, `t.skip()` com razão nomeada
que diga explicitamente "condição de ambiente" ou "não defeito de produto".

Referência: REQ-2026-09-12-suites-de-teste-nao-distinguem-ambiente-incompleto-de-codigo-quebrado.
Ver também: [[ambiente-do-dev-e-mais-rico-que-o-do-ci-2026-08-29]] para o padrão geral.
