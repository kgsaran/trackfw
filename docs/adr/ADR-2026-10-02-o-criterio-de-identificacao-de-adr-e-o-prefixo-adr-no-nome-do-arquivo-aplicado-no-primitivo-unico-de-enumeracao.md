---
status: Accepted
date: 2026-10-02
author: "trackfw_architect"
---

# ADR: o critério de identificação de ADR é o prefixo `ADR-` no nome do arquivo, aplicado no primitivo único de enumeração

> Date: 2026-10-02 | Status: Accepted

REQ: `docs/req/REQ-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md`
Origem: **#471** (com medição de consumidor externo no comentário)
Revoga: a decisão de "não mudar o critério" registrada nas Consequências da
`ADR-2026-09-29-a-enumeracao-de-adr-e-uniao-de-layouts-e-context-e-status-consomem-o-mesmo-ponto-unico.md`.
Mantém: o ponto único de leitura (D3 daquela ADR), que é onde este critério passa a viver.

## Context

O critério de identificação de ADR é `strings.HasSuffix(path, ".md")`, sem filtro. Qualquer `.md` em
`adr_dirs` (`README.md`, `NOTAS.md`, `index.md`) conta como ADR:

- `status` e `context` contam o arquivo;
- `adr list` o lista com status `unknown`, o que parece defeito de frontmatter;
- o `validate` acusa `adr "NOTAS.md" has no frontmatter block`, quando o caso real é "isto não é ADR";
- o `discover` dá **+20** de Governance Score ao fantasma. É o mesmo crédito de um ADR real, na direção
  que induz confiança na ausência de problema.

**Medição do custo** (comentário da #471, 6 acervos, 133 ADRs):

| critério | conta | deixa de contar |
|---|---|---|
| atual (todo `.md`) | 133 | — |
| prefixo `ADR-` | 133 | **0** |
| frontmatter válido | 132 | 1 (o `ADR-001`, que usa `**Status:**` no corpo) |

**Onde o critério vive hoje:** um primitivo só, `walkADRFilePathsForRule` (`internal/validator/validator.go`),
alimenta `ResolveADRFiles` (status, context, discover pelo caminho declarado), `WalkADRFilePaths`
(`adr list`, numeração do `adr new`) e as regras do `validate`. **Fora dele:** `scanChainDir` do
`serve` (`internal/serve/api_chain.go`, grafo de rastreabilidade) e a sonda de fallback do `discover`
(`countMDFiles` sobre `docs/adr` quando não há declaração).

## Decision

### D1 — Um arquivo é ADR quando o basename começa com `ADR-`, sem distinção de maiúsculas, e termina em `.md`.

É o que o gerador emite (`trackfw adr new`) e o que os 6 acervos medidos já seguem. A
insensibilidade a maiúsculas aceita `adr-001.md` sem custo. Frontmatter **não** entra no critério: a
validade do frontmatter continua sendo trabalho das regras do `validate`, sobre o que já é ADR.

### D2 — O critério vive em `walkADRFilePathsForRule`, e só lá.

Todo consumidor que enumera ADR passa por ele: `ResolveADRFiles`, `WalkADRFilePaths` e
`findADRFile`. Mudar o critério ali move contagem, listagem, numeração e regras **no mesmo gesto**.
O `NOTAS.md` deixa de ser contado **e** deixa de ser violação de frontmatter.

### D3 — Os dois sítios de fora passam a usar o primitivo.

- `serve` (`scanChainDir`, tipo `adr`) enumera os nós de ADR pelo primitivo, não pela própria varredura de `.md`.
- `discover`, na sonda de fallback sem declaração, conta ADR pelo primitivo, não por `countMDFiles`.

O primeiro caminho do `discover` (diretório declarado) já usa `ResolveADRFiles` desde o #498.
Pela Regra Dura de Causa Raiz, o ponto único não está satisfeito enquanto sobrar sítio.

### D4 — Aviso para o caso que o critério esconde: `.md` com cara de ADR e sem prefixo.

O risco do D1 é um ADR real, fora do padrão de nome, sumir em silêncio. A regra nova
`adr_file_without_prefix` (severidade **warning**) acusa, dentro de `adr_dirs`, o `.md` **sem**
prefixo `ADR-` cujo frontmatter declara `status:`. A mensagem diz que o arquivo não é contado como
ADR e que deve ser renomeado para `ADR-…` se for um.

Os `README.md`/`index.md` sem frontmatter **não** disparam o aviso. Ele existe para o ADR mal nomeado,
não para reclamar de documento auxiliar.

## Consequences

- `status`, `context`, `adr list`, `validate`, `discover` e `serve` passam a concordar sobre o que é ADR.
- O Governance Score do `discover` deixa de creditar `.md` avulso.
- **Mudança de comportamento para consumidor:** um ADR sem prefixo deixa de contar. O custo medido em
  6 acervos é zero, e o D4 avisa no caso que importa.
- **Resíduo declarado:** um ADR sem prefixo **e** sem frontmatter some sem aviso. Esse arquivo também
  não é reconhecível como ADR por nenhum outro critério barato.

## Alternatives Considered

- **Frontmatter válido como critério.** Rejeitado pelo KG em 2026-10-02. Seria mais correto
  semanticamente, mas derruba o `ADR-001` fundador e mistura identificação com validação: um ADR com
  frontmatter quebrado sumiria em vez de ser acusado.
- **Prefixo E frontmatter.** Soma os dois custos.
- **Declarar o comportamento atual.** Mantém o score falso e a mensagem enganosa do `validate`.

## Linked REQ

`docs/req/REQ-2026-10-02-qualquer-md-em-adr-dirs-e-contado-como-adr-o-criterio-passa-a-ser-o-prefixo-adr.md`
