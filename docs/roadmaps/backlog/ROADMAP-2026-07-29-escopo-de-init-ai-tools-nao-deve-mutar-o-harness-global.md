---
status: backlog
date: 2026-07-29
req: "REQ-2026-07-29-escopo-de-init-ai-tools-nao-deve-mutar-o-harness-global"
squad: ""
---

# Roadmap: Escopo de init --ai-tools nao deve mutar o harness global

> Created: 2026-07-29 | Status: backlog

## Context
<!-- What problem does this roadmap solve? Link the REQ. -->
REQ: 

## Wave 1 — <name> (parallel MLs)
> Dependencies: none

### ML-1A — Escopo de init --ai-tools nao deve mutar o harness global
**Status:** pending
**Files affected:**
**Actions:**
**Acceptance criteria:**
- [ ] build passes
- [ ] tests green
- [ ] validate passes

## Critérios de Aceite

- [ ] `trackfw init --ai-tools <tool>` não escreve fora do diretório do projeto.
- [ ] Instalação global passa a exigir escopo explícito, coerente com `trackfw update harness`.
- [ ] Teste com HOME isolado prova a ausência de escrita global nos três runtimes.
- [ ] Gate encadeado no alvo `parity` prova a ausência de escrita global.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Wave 1 — Restringir o escopo de `init --ai-tools` (1 ML)
> Dependências: nenhuma

**Gates da wave:**
```bash
make quality
bin/trackfw validate --json
```

### ML-1A — Restringir `init --ai-tools` ao escopo do projeto
**Status:** ⬜ Pendente
**Origem:** constatado pelo orquestrador ao validar a regressão do ML-5E.
**Arquivos afetados:**
- caminho de `init --ai-tools` nos três runtimes
- testes correspondentes

**Diagnóstico:** `trackfw init --ai-tools gemini`, executado dentro de um projeto, grava em
`~/.gemini/agents/`. Constatado empiricamente: a execução falhou com
`artifact "/Users/<user>/.gemini/agents/trackfw-architect.md" is outdated; use update`, provando
que o comando alcança o HOME do usuário.

É a **mesma classe de defeito** que a Wave 6 corrige em `trackfw update`: um comando de escopo de
projeto mutando o harness global. O contrato do ML-6A cobre `update`; `init` ficou de fora.

**Ações:**
1. Restringir `init --ai-tools` ao escopo do projeto, seguindo o contrato do ML-6A.
2. Instalação global passa a exigir escopo explícito, coerente com `trackfw update harness`.
3. Teste com HOME isolado provando que `init` não escreve fora do projeto.

**Critérios de aceite:**
- [ ] `init --ai-tools` não escreve fora do diretório do projeto.
- [ ] Teste com HOME isolado prova a ausência de escrita global.
- [ ] Comportamento idêntico nos três runtimes.
- [ ] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

**Comandos de validação:**
```bash
make quality
```

