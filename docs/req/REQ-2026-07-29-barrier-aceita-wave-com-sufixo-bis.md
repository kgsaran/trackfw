---
status: Done
date: 2026-07-29
author: "trackfw_architect"
adr: "docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md"
roadmap: "docs/roadmaps/done/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md"
---

# REQ: barrier aceita wave com sufixo bis

> Date: 2026-07-29 | Status: Done
| Linear Issue:
| Jira Issue:

## Motivation

Waves corretivas são um padrão real, não hipotético. No roadmap
`install-pula-artefato-desatualizado-em-vez-de-abortar`, a auditoria cruzada da Wave 2 revelou que o
contrato estava incompleto e exigiu uma wave de convergência **acrescentada depois** que a Wave 2 já
havia sido executada e commitada. A nomenclatura natural para isso é `Wave 2-bis`: sinaliza "corrige a
Wave 2" e preserva a numeração das waves seguintes já referenciadas em commits.

O `trackfw barrier` rejeita:

```
trackfw barrier: malformed wave heading at line 250: "2-bis" is not a valid wave number
```

E o efeito é pior que a rejeição em si: **as quatro waves falharam**, não apenas a malformada. O
parser varre todas as headings procurando a wave alvo (`internal/commands/barrier.go:146`,
`waveHeadingRe = ^## Wave (\S+) `) e levanta o erro ao encontrar qualquer token não-inteiro, antes de
decidir se aquela heading é a que interessa. Uma heading inválida em qualquer lugar do documento torna
o roadmap inteiro não-auditável.

Contorno aplicado na ocasião: renumerar para inteiros (Wave 3 corretiva, Wave 4 auditoria) e reordenar
os blocos. Funcionou, mas dessincronizou os ids dos MLs das waves (`ML-2D`/`ML-2E` na Wave 3), porque
renumerar os ids quebraria a rastreabilidade de mensagens de commit já publicadas.

## Decisão de design que mudou durante a análise — não reverter

A hipótese inicial incluía **tornar a heading malformada não-fatal**, escopando o erro à wave
solicitada em vez de abortar o documento. **Isso foi descartado, e o descarte é deliberado.**

Se uma heading malformada for silenciosamente ignorada, os MLs contidos nela **deixam de ser
auditados** e a wave passa sem que ninguém veja. Uma heading com typo (`## Wave X — ...`) produziria
uma barrier verde sobre trabalho não verificado. O comportamento de abortar alto é **uma feature**: o
parser não pode escolher entre "ignorar o que não entende" e "reprovar", porque a primeira opção é
vacuosa — exatamente a classe de falso positivo que o
`vault/notes/barrier-contract-xfail-false-positive-2026-07-29.md` documenta.

Portanto o escopo é **apenas admitir o sufixo na gramática**, não relaxar a rigidez do parser. Uma
heading que não casar com a gramática estendida continua abortando o documento inteiro.

## Divergência de paridade encontrada na análise

As três mensagens de heading malformada **já divergem hoje**, porque o contrato pina apenas duas
mensagens de exit-2 (`roadmap not found` e `wave not found`) e esta ficou de fora:

| Runtime | Mensagem atual |
|---|---|
| Go (`barrier.go:183`) | `malformed wave heading at line %d: %q is not a valid wave number` |
| Node.js (`barrier.js:59`) | `malformed wave heading at line ${i+1}: "${line}"` — a linha inteira, sem dizer o motivo |
| Python (`barrier.py:116`) | `malformed wave heading at line {i+1}: number {token!r} is not parseable` |

Três textos, um deles sem informar a causa. Pelo raciocínio já registrado no próprio contrato — "um
runtime que parafraseia satisfaz seus próprios testes e quebra a equivalência entre runtimes" — esta
terceira mensagem precisa ser pinada literalmente.

## Acceptance Criteria

- [x] Gramática do rótulo de wave estendida para `<inteiro>[-<sufixo>]`, com sufixo em minúsculas
      (`[a-z0-9]+`). Válidos: `2`, `2-bis`, `2-hotfix`. Inválidos: `X`, `2-BIS`, `-bis`, `2-`.
- [x] `--wave` aceita o rótulo verbatim (`--wave 2-bis`) e continua aceitando inteiro (`--wave 2`).
      `--wave 2` **não** casa com `Wave 2-bis`: são rótulos distintos.
- [x] Ordenação pinada: `2-bis` ordena imediatamente após `2` e antes de `3`; sufixos entre si ordenam
      lexicograficamente.
- [x] Heading fora da gramática estendida **continua abortando o documento inteiro** — preservado por
      teste explícito, para que ninguém o "corrija" depois.
- [x] Terceira mensagem de exit-2 pinada literalmente e byte-idêntica nos três runtimes.
- [x] `docs/cli-parity.md` atualizado: linha `--wave` da tabela de command surface, regra 1 de parsing
      e o bloco de mensagens pinadas.
- [x] Paridade nos três CLIs, com cenário de comparação byte-a-byte encadeado em `make quality`.
- [x] `make quality` passa e `bin/trackfw validate --json` retorna 0 violações.

## Linked ADR
ADR: `docs/adr/ADR-2026-07-29-barrier-governanca-e-autoridade-do-orquestrador.md`

Muda o contrato do `barrier` — `--wave` deixa de ser "Integer ≥ 1" e a regra 1 de parsing deixa de
exigir inteiro. Exige emenda ao ADR **antes** da implementação.

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: `docs/roadmaps/done/ROADMAP-2026-07-29-barrier-aceita-wave-com-sufixo-bis.md`
