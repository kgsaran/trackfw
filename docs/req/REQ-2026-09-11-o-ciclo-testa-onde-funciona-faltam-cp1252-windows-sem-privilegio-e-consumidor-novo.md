---
status: Open
date: 2026-09-11
author: ""
adr: ""
roadmap: "docs/roadmaps/done/ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md"
---

# REQ: o ciclo testa onde funciona: faltam cp1252, Windows sem privilegio e consumidor novo

> Date: 2026-09-11 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation
<!-- Why is this requirement needed? What problem does it solve? -->

## Acceptance Criteria
- [ ]
- [ ]

## Linked ADR
<!-- Reference the ADR that governs this requirement -->
ADR: 

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
<!-- Reference the roadmap that implements this requirement -->
Roadmap: `docs/roadmaps/done/ROADMAP-2026-09-11-o-ciclo-testa-onde-funciona-faltam-cp1252-windows-sem-privilegio-e-consumidor-novo.md`
## Motivation

Pergunta do usuário em 2026-09-11: *"por que o Lourival está achando tantos erros e nós não?"*

Análise completa em `docs/qualidade/2026-09-11-por-que-o-consumidor-externo-acha-e-nos-nao.md`.
**A resposta medida não foi "falta teste" — foi "testamos onde funciona".**

### O que a medição mostrou

```
nosso CI:  18 jobs ubuntu-latest  ·  6 jobs windows-latest
```

| issue | ambiente do defeito | cobrimos? |
|---|---|---|
| **#314** | console `cp1252` (Windows pt-BR) | 🔴 não, no caminho do self-test |
| **#315** | Windows **sem privilégio de symlink** | 🔴 não — o runner tem Developer Mode |
| **#320** | projeto `by_agent` com **2 agentes** | 🔴 não — **zero** testes com 2+ agentes |

🔴 **Em nenhum dos três o defeito estava num caminho que exercitamos.** E os três são **configuração
trivial** do ponto de vista de quem usa — não casos exóticos.

O `#279` e o `#315` são a mesma classe em arquivos diferentes: a varredura original fechou com o
5º sítio vivo, porque **nada exercita o ambiente onde ele aparece**.

## Por que gate, e não revisão

Estes três são **ambientes**, não julgamentos. Uma vez ligados, pegam **para sempre** e não dependem
de ninguém lembrar de olhar.

🔴 É a diferença entre corrigir a instância e fechar a classe — a mesma lição do `check-orphan-gates`
hoje: dois gates estavam inertes, e só o gate da classe impede o terceiro.

## Acceptance Criteria

- [ ] **AC1 — console `cp1252`.** Um passo de CI em `windows-latest` roda a suíte de gates com
      codepage **1252**. 🔴 **Falsificação obrigatória:** com o passo ligado e a correção do `#314`
      revertida, ele **reprova** — senão não se sabe se o passo exercita algo.

- [ ] **AC2 — Windows SEM Developer Mode.** Um job que exercite o caminho de `os.Symlink` **sem** o
      privilégio. 🔴 **Meça primeiro se é possível no runner hospedado**: o Developer Mode vem ligado,
      e desligá-lo pode não ser permitido. Se não for, **declare a limitação** e proponha o
      substituto (ex.: simular a negação de privilégio), **nunca finja cobertura**.

- [ ] **AC3 — consumidor novo.** Um smoke que faz `init` num **projeto descartável**, com
      `roadmap_namespacing: by_agent` e **2 agentes**, e roda os comandos principais nos 3 CLIs.
      Deve pegar o `#320` (artefato sempre no primeiro agente; `--req` ignorando o agente da REQ).

- [ ] **AC4 — 🔴 job verde com anotação de erro reprova.** O `#319` é um job `success` com 10
      anotações `failure`. Nossa disciplina é "deixar verde" e paramos de olhar depois disso.
      Gate que compare conclusão com anotações fecha esse ponto cego.

- [ ] **AC5 — re-mutação de gate antigo.** Alvo (à la `check-required-full`) que aplique mutação nos
      gates **existentes** e confira que acusam. Um gate correto no dia 1 vira vacuoso no dia 30 por
      mudança adjacente, e hoje **nada percebe** — foi o `#309`.

- [ ] **AC6 — 🔴 cada gate novo precisa de contra-braço.** Ambiente que nunca reprova é indistinguível
      de ambiente que não exercita nada. Vale para os três de AC1-AC3.

- [ ] **AC7 — custo declarado.** Estes jobs acrescentam tempo a todo PR. **Meça e escreva** o custo;
      se for proibitivo, a alternativa é amarrar ao release, como fizemos com o `check-required-full`
      — mas a decisão fica **escrita**, não implícita.

## Fora desta REQ

O papel de **QA como consumidor adversarial** (agente que roda o artefato em ambiente hostil, como
projeto novo, mutando o que passa, com contexto mínimo nosso). É mudança de **processo**, não de
código, e merece ADR própria.

🔴 **E há um limite que nenhum gate desta REQ resolve:** quem audita já sabe o que esperava
encontrar. Está registrado na análise; a mitigação conhecida é a instrução de *reimplementar a partir
da leitura*, que é o que faz o `hades-tf` funcionar.
