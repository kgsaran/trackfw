---
title: Promise flutuante em `.action()` do CLI Node é invisível na fronteira do processo — e o teste que importa+mocka não vê o defeito de `await` faltando
tags: [node, async, cli, testes, gotcha, falsificacao]
date: 2026-09-02
related: [[paridade-cross-runtime-dentro-do-go-test-quebra-o-job-go-2026-08-29]]
---

## Sintoma

`node npm/bin/trackfw context` → exit 1, `Error: Cannot read properties of undefined (reading
'length')`. O comando **nunca** funcionou: `validate` é `async function`
(`npm/src/validator/index.js`), e `context.js` desestruturava a Promise sem `await`, então
`violations` era `undefined` na linha seguinte. Go e Python nunca tiveram o defeito.

## Causa Raiz

O defeito mora na **fronteira** entre o comando e o validator — dentro de nenhum dos dois. Nenhum
teste do pacote npm executava o **binário** do `context`; o teste existente
(`context_req_by_agent.test.js`) *reimplementa* `collectEntries`/`collectReqs` no próprio arquivo de
teste, justamente para não invocar o comando. Um teste que importasse `context.js` e mockasse
`validate` com uma função síncrona teria passado nas duas árvores — o mock **é** o que esconde a
classe inteira.

## O achado não óbvio (é isto que se re-deriva do zero amanhã)

A correção completa tem 3 partes: `async function getContext`, `await validate()`, e **retornar** a
Promise da `.action()`. As duas primeiras são falsificáveis pelo binário. A terceira **não é**:

- **Caminho feliz:** sem o `return`, a Promise flutua, mas o event loop drena antes de o processo
  sair — a saída é impressa e o exit é 0, igual à árvore correta.
- **Caminho de erro:** com `trackfw.yaml` malformado, a árvore **com** e a **sem** o `return` saem
  ambas com exit 1 e a mesma mensagem — porque `installGlobalHandlers()` em `npm/bin/trackfw`
  converte a unhandled rejection em erro reportado.

**Consequência geral, além do `context`:** em qualquer comando do CLI Node, esquecer de propagar a
Promise da `.action()` é **observacionalmente indistinguível** na fronteira do processo. Não existe
teste de CLI que pegue isso; o handler global mascara. Quem quiser cobrir precisa de teste em nível
de módulo sobre o valor retornado pela `.action()`, não de execução do binário.

## Remédio

1. `.action((opts) => getContext(opts.format))` — devolver a Promise, sem depender do handler global.
2. Teste que **executa o binário** (`spawnSync`) em projeto de governança mínimo sob `mktemp`, com
   `HOME`/`USERPROFILE` isolados (o `validate` lê regras de credential-guard em escopo global — ver
   [[check-agent-hooks-parity-unisolated-home-false-failure-2026-08-08]]).
3. Falsificação medida nas duas direções: removendo só o `await`, `npm test --prefix npm` sai **1**.

## Varredura da classe (2026-09-02, resultado: zero outros achados)

33 `async function` nomeadas em `npm/src`+`npm/bin`; nenhum `const x = async`, nenhum método
`async nome(`, nenhuma atribuição `prop = async`, nenhum `forEach/map(async ...)`. Excluídas as 6 de
`npm/src/serve/static/app.js` (código de browser). Nas 27 restantes, todo call site está `await`ado
ou propaga a Promise por `return`. `getStatus` — a irmã de `validate` — tem um único chamador,
`npm/src/commands/status.js:9`, `await`ado. Atenção ao ler grep de `validate(`: existe um `validate`
**síncrono** e homônimo em `npm/src/identity/config.js:102`, chamado sem `await` de propósito.
