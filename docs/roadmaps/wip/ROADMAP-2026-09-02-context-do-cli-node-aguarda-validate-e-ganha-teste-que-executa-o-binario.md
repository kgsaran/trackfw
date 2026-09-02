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
**Status:** ⬜ Pendente
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
- [ ] `node npm/bin/trackfw context` sai 0 e imprime o contexto, verificado por execução
- [ ] 🔴 **Falsificação:** removendo o `await`, o teste novo **reprova**. Um teste que passa nas duas
      árvores não mede nada
- [ ] 🔴 **Controle:** a saída do `context` no Go e no Python **não muda** — comparar antes/depois
- [ ] A varredura da classe está feita e o resultado registrado (inclusive se for zero)
- [ ] `make quality` verde

**Comandos de validação:** `node npm/bin/trackfw context`, `npm test --prefix npm`, `make quality`

## Verificação

O comando funcionando é observável direto. O que só o teste fecha é a **não-regressão**: o defeito
sobreviveu desde a origem por ausência de teste que rodasse o CLI.

## Barreira final

Arquiteto. **Sem `hades-tf`** — não há superfície de ataque. `hefesto-tf` só se a varredura da classe
virar refactor.
