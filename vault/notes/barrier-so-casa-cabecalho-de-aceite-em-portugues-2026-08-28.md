---
title: barrier reprova todo roadmap que o próprio trackfw gera — cabeçalho de aceite em idiomas diferentes
tags: [barrier, roadmap, gate, gotcha, paridade]
date: 2026-08-28
related: [[roadmap-title-newline-forges-wave-section-barrier-executes-gate-2026-08-23]]
---

## Sintoma

`trackfw barrier <roadmap> --wave N` reprova com:

```
✗ acceptance_evidence: blocked
    - ML-3A: no acceptance block
```

num ML que **tem** o bloco de aceite, com todos os critérios marcados `[x]`. A mensagem diz que o
bloco não existe, então o instinto é procurar erro de formatação nos critérios — e não há nenhum.

## Causa Raiz

O **gerador** e o **verificador** usam idiomas diferentes para o mesmo cabeçalho:

| | escreve / procura | onde |
|---|---|---|
| `roadmap new` (Go) | escreve `**Acceptance criteria:**` | `internal/generators/roadmap.go:64,176,225` |
| `roadmap new` (Node) | escreve `**Acceptance criteria:**` | `npm/src/generators/roadmap.js:31,495,558` |
| `roadmap new` (Python) | escreve `**Acceptance criteria:**` | `pypi/trackfw/generators/roadmap.py:40` |
| `barrier` (Go) | procura `^\*\*Crit[eé]rios de aceite:\*\*` | `internal/commands/barrier.go:166` |
| `barrier` (Node) | procura `/^\*\*Crit[ée]rios de aceite:\*\*/` | `npm/src/commands/barrier.js:144` |
| `barrier` (Python) | procura `^\*\*Crit[ée]rios de aceite:\*\*` | `pypi/trackfw/commands/barrier.py:105` |

Ou seja: **todo roadmap criado pelo `trackfw roadmap new` é reprovado pelo `barrier`**, em qualquer
um dos 3 CLIs, até alguém editar o cabeçalho à mão.

## Por que nenhum gate pega

A **paridade entre os 3 CLIs está intacta** — os três geradores escrevem inglês, os três barriers
procuram português. Os gates de paridade medem se as implementações concordam **entre si**, e elas
concordam perfeitamente. O que está quebrado é o contrato **gerador ↔ verificador**, e não há gate
que atravesse essa fronteira.

Lição generalizável: *paridade entre implementações não é o mesmo que correção do contrato.* Um
conjunto de gates que só compara runtimes é cego para erro cometido igualmente nos três.

## Contorno enquanto não houver correção

Trocar, no roadmap, `**Acceptance criteria:**` por `**Critérios de aceite:**`:

```bash
sed -i '' 's|^\*\*Acceptance criteria:\*\*$|**Critérios de aceite:**|' docs/roadmaps/wip/<arquivo>.md
```

## Como foi descoberto

Usando a ferramenta em trabalho real, não por inspeção de código: a barreira da Wave 3 da
`REQ-2026-08-28-gate-de-ci-gerado-instala-versao-nao-pinada-do-trackfw-e-nao-ha-como-pinar`
reprovou um ML que eu sabia estar completo. A suíte de 181 cenários de falsificação não pega, pelo
motivo acima.

## Rastreamento

`REQ-2026-08-28-barrier-so-reconhece-cabecalho-de-aceite-em-portugues-mas-os-3-geradores-de-roadmap-escrevem-em-ingles.md`
(aberta, `status: Open`). Ela exige que a forma portuguesa **continue** aceita — há roadmaps em
`done/` e `wip/` com ela — e que um ADR decida qual é a forma canônica daqui pra frente.
