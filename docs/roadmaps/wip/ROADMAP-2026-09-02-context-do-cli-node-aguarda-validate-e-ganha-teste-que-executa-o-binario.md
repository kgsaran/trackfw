---
status: wip
date: 2026-09-02
squad: apolo-tf
req: "docs/req/REQ-2026-08-30-trackfw-context-do-cli-node-falha-sempre-porque-validate-assincrono-e-chamado-sem-await.md"
---

# Roadmap: `context` do CLI Node aguarda `validate` e ganha teste que executa o binário

> Criado em: 2026-09-02 | Status: wip

## Context

REQ: docs/req/REQ-2026-08-30-trackfw-context-do-cli-node-falha-sempre-porque-validate-assincrono-e-chamado-sem-await.md

## Diagnóstico

`npm/src/commands/context.js:136` faz `const { violations, warnings } = validate()`. `validate` é
`async function` (`npm/src/validator/index.js:3237`), então a desestruturação de uma Promise devolve
`undefined` e a linha seguinte estoura.

**Reproduzido pelo arquiteto em 2026-09-02, na `main`:**

```
$ node npm/bin/trackfw context
Error: Cannot read properties of undefined (reading 'length')
```

**O comando nunca funcionou.** `async function validate()` existe desde a reescrita original do
pacote npm — não é regressão, é defeito de origem.

🔴 **E o `context` é o primeiro comando do protocolo de agentes:** *"Before starting: run
`trackfw context`"*. Todo agente operando pelo CLI Node bate nisso na primeira instrução.

## Diagnóstico da causa raiz — e é ela que decide o escopo

A correção é **uma palavra** (`await`). O trabalho real é responder: **por que isto sobreviveu desde
a origem?**

Porque nenhum teste executa o **binário**. Um teste que importa o módulo e faz mock do `validate`
não teria pego — o defeito está na fronteira entre o comando e o validator, não dentro de nenhum dos
dois. É o mesmo cego já registrado em
`vault/notes/paridade-cross-runtime-dentro-do-go-test-quebra-o-job-go-2026-08-29.md`.

## Status Legend
⬜ Pendente · 🔄 Em andamento · ✅ Concluído · ❌ Bloqueado

## Acceptance Criteria

- [ ] `node npm/bin/trackfw context` executa e imprime o contexto de governança
- [ ] 🔴 Teste que executa o **binário de verdade**, não o módulo com mock
- [ ] 🔴 Varredura por outras chamadas de função `async` sem `await` no CLI Node
- [ ] Paridade verificada: Go e Python não têm o defeito
- [ ] `make quality` verde

## Wave 1 — A correção e o teste que a teria pegado
> Dependências: nenhuma.

### ML-1A — `await` no `context` e teste pelo binário
**Status:** ✅ Concluído
**Agente:** `apolo-tf`
**Files affected:** `npm/src/commands/context.js`, teste novo em `npm/tests/`

**Ações:**
1. `await validate()` em `context.js:136`.
2. **Teste que executa `npm/bin/trackfw context` como processo** e assere na saída. Sem mock do
   `validate` — o mock é justamente o que esconderia o defeito de novo.
3. 🔴 **Varredura da classe, não só da instância:** procurar outras chamadas a funções `async` sem
   `await` no `npm/src/`. A REQ registra `grep -rn "validate()" npm/src npm/bin | grep -v await` →
   só uma linha, mas **essa varredura cobre só `validate`**. Enumerar as demais `async function` do
   pacote e verificar cada chamador. **Zero achados é resultado válido** e encerra o item.

**Critérios de aceite:**
- [x] `node npm/bin/trackfw context` sai 0 e imprime o contexto, verificado por execução
- [x] 🔴 **Falsificação:** removendo o `await`, o teste novo **reprova**. Um teste que passa nas duas
      árvores não mede nada
- [x] 🔴 **Controle:** a saída do `context` no Go e no Python **não muda** — comparar antes/depois
- [x] A varredura da classe está feita e o resultado registrado (inclusive se for zero)
- [x] `make quality` verde


**Evidência de aceite — auditoria do arquiteto (2026-09-02), reproduzida de forma independente:**

```
arvore corrigida  node npm/bin/trackfw context          -> rc=0, contexto impresso
teste novo        node --test npm/tests/context_cli...  -> rc=0
SABOTADO (só o await removido)                          -> rc=1  <- discrimina
restaurado                                              -> rc=0
```

`make quality` → `QUALITY_EXIT=0`, zero `FAIL`. Controle: **nenhum arquivo em `internal/` ou
`pypi/` alterado** — Go e Python não foram tocados, verificado por `git diff --name-only`.

🔴 **A correção é de 3 linhas, não de uma palavra — e a terceira não estava no meu handoff.** Sem
`return` da Promise no `.action()`, a `getContext` vira `async` e a Promise fica **fora** do
`parseAsync().catch(reportFatalError)` do `npm/bin/trackfw`: uma rejeição deixaria de virar exit 1.

🔴 **Residual declarado pela agente, e a honestidade aqui vale o registro:** essa terceira linha
**não é falsificável pelo binário**. Sabotando só ela, o teste passa — no caminho feliz o event loop
drena a Promise flutuante, e no caminho de erro o `installGlobalHandlers()` captura a unhandled
rejection e sai 1 igual. As duas árvores foram medidas e a saída é idêntica. Mantida por robustez,
com o motivo escrito no código, e registrada em
`vault/notes/promise-flutuante-em-action-do-cli-node-e-invisivel-na-fronteira-2026-09-02.md` porque
**generaliza**: em qualquer comando do CLI Node, esquecer de propagar a Promise do `.action()` é
invisível na fronteira do processo.

**Varredura da classe — enumeração, não conclusão.** 33 `async function` em `npm/src`+`npm/bin`;
6 excluídas com justificativa (código de browser em `serve/static/app.js`, inalcançável pelo bin);
27 classificadas uma a uma — 19 com call site `await`ado, 6 com Promise propagada por `return`, 6 sem
chamador (entrypoints de retrocompatibilidade). **Zero achados**, com a lista disponível para
auditar a varredura em vez de acreditar nela.

**Efeito colateral medido:** a saída md do Node ficou **byte-idêntica à do Go** — `diff` vazio,
425 linhas.

**Fora de escopo, declarado e não corrigido:** no `--format json` o Go emite `"violations": null`
(slice nil) onde Node e Python emitem `[]`; e o Python ordena as listas de ADR/REQ de forma diferente
de Go e Node. Ambos pré-existentes, sem efeito no score — candidatos a REQ própria.

**Comandos de validação:** `node npm/bin/trackfw context`, `npm test --prefix npm`, `make quality`

## Verificação

O comando funcionando é observável direto. O que só o teste fecha é a **não-regressão**: o defeito
sobreviveu desde a origem por ausência de teste que rodasse o CLI.

## Barreira final

Arquiteto. **Sem `hades-tf`** — não há superfície de ataque. `hefesto-tf` só se a varredura da classe
virar refactor.
