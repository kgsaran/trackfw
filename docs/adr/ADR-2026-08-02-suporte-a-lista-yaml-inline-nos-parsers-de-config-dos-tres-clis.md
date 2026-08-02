---
status: Accepted
date: 2026-08-02
author: "Zeus"
---

# ADR: Suporte a lista YAML inline nos parsers de config dos tres CLIs

> Date: 2026-08-02 | Status: Accepted

## Context

Último item da fila. Os **três** CLIs ignoram silenciosamente a forma inline (flow-style) de
lista YAML:

```yaml
agents: [zeus, apolo]
adr_dirs: [docs/adr, docs/adr2]
```

O usuário escreve YAML válido e **nada acontece** — sem erro, sem aviso. A chave é tratada como
escalar, não casa nenhuma lista, e o CLI cai no default ou no fallback.

Diferente dos defeitos anteriores deste ciclo, este é **consistente** entre os três CLIs, portanto
não é problema de paridade. É defeito de produto: configuração válida descartada em silêncio, que
é a pior classe de erro de config — o usuário não tem sinal de que errou.

### Estado dos parsers

Os três parsers de config são **artesanais**, sem biblioteca YAML:

| CLI | Dependência YAML |
|---|---|
| Go | `gopkg.in/yaml.v3` existe no `go.mod` mas é **indirect** — não usada pelo config |
| Node | duas deps de runtime (`commander`, `@inquirer/prompts`), nenhuma de YAML |
| Python | nenhuma dependência |

Todos reconhecem apenas a forma em bloco (`- item`).

### Chaves afetadas

`adr_dirs`, `agents`, `acceptance_markers`, e as sub-listas de `link_fields`
(`req`, `adr`, `roadmap`). `rules` é mapeamento, não sequência — fora.

## Decision

**Os três parsers passam a aceitar a forma inline, em todas as chaves de lista.**

A semântica é idêntica à da forma em bloco: a lista substitui o default, e uma lista vazia é
lista vazia — não ausência.

### Especificação exata do parsing

Isto é **contrato entre os três CLIs**, não sugestão de implementação. A tabela abaixo é a
definição do comportamento; qualquer divergência é defeito.

| # | Entrada | Resultado |
|---|---|---|
| 1 | `[a, b]` | `[a, b]` |
| 2 | `[a,b]` | `[a, b]` |
| 3 | `[ a , b ]` | `[a, b]` |
| 4 | `["a", "b"]` | `[a, b]` |
| 5 | `['a', 'b']` | `[a, b]` |
| 6 | `[a]` | `[a]` |
| 7 | `[]` | `[]` (lista vazia, **não** default) |
| 8 | `["a, b", "c"]` | `[a, b]` com **dois** itens: `a, b` e `c` |
| 9 | `["## Acceptance Criteria", "## Critérios de Aceite"]` | os dois marcadores |

**O caso 8 é o que exige atenção.** Separar por vírgula ingenuamente quebra valores entre aspas
que contêm vírgula. O parser precisa respeitar as aspas ao separar. O caso 9 é o caso real:
`acceptance_markers` contém `## Critérios de Aceite`, e um valor com vírgula é plausível.

### Regras adicionais

- Espaços em torno dos itens e dos colchetes são descartados.
- Aspas simples e duplas são removidas do item, como já é feito na forma em bloco.
- Se a mesma chave aparecer nas duas formas no mesmo arquivo, o comportamento é **indefinido** e
  não é objeto deste ADR — não há caso real, e definir precedência sem necessidade é especulação.

### Como o risco de divergência é controlado

Este projeto mostrou, em **todos** os ciclos, que três implementações paralelas divergem — em
fonte de dado, em texto, em raio de alcance. Duas medidas:

1. **Um único executor** implementa nos três CLIs. Coordenar três agentes para produzir semântica
   idêntica provou ser mais frágil do que um executor com a tabela na mão.
2. **Cenário de falsificação por caso da tabela**, nos três CLIs, com braço de detecção.

## Consequences

**Positivas**

- Elimina a classe "config válida descartada em silêncio" para listas.
- A forma inline é a mais compacta e a mais usada em configs curtos — `adr_dirs: [docs/adr]` é
  mais natural que três linhas.
- Fecha o último item da fila, permitindo tagear sem defeito conhecido.

**Negativas / aceitas**

- **Amplia a superfície dos três parsers artesanais.** É a objeção real: mais código de parsing
  duplicado em três linguagens, com histórico de divergir. Aceito por decisão de KG, e mitigado
  por executor único e falsificação por caso.
- O parser continua sendo um subconjunto de YAML. Suportar inline **não** o torna um parser YAML —
  formas como listas aninhadas inline (`[[a],[b]]`) ou mapas inline (`{a: 1}`) seguem sem suporte
  e sem aviso. O defeito de classe é reduzido, não eliminado.

## Alternatives Considered

**Emitir aviso de "lista inline ignorada"** — superfície mínima, risco quase nulo.
**Rejeitado por KG:** acaba com o silêncio, mas o usuário escreveu YAML válido e o CLI continua
não obedecendo. É consolo, não correção. Ainda exigiria três mensagens novas em paridade.

**Adotar biblioteca YAML de verdade nos três** — eliminaria a classe inteira, inclusive as formas
que continuarão sem suporte. **Rejeitado:** no Go seria barato (`yaml.v3` já está no grafo como
indirect), mas no Node e no Python significa adicionar dependência de runtime e reescrever o
parsing de config inteiro. É mudança de política de dependências e de arquitetura — grande demais
para um ciclo cujo objetivo é esvaziar a fila antes da tag. **Continua sendo a solução certa a
prazo**, e deve virar ADR próprio se o parser artesanal voltar a dar problema.

**Suportar inline só em `agents`**, que foi onde o problema apareceu — **Rejeitado:** o defeito
está no laço do parser, não numa chave. Corrigir uma deixaria as outras quatro com o mesmo
silêncio, e a próxima pessoa a tropeçar teria menos contexto que nós agora.
