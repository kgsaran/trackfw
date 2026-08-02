---
status: Open
date: 2026-08-02
author: "Zeus"
adr: "docs/adr/ADR-2026-08-02-python-alinha-delimitador-nao-pareado-e-ordenacao-do-fallback-de-agentes.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-parsing-remanescentes-no-python.md"
---

# REQ: Fechar as duas divergencias de parsing remanescentes no Python

> Date: 2026-08-02 | Status: Open
| Linear Issue: 
| Jira Issue: 

## Motivation

Os dois últimos itens da fila, ambos medidos e sem caso real no repositório. KG pediu que fossem
fechados **antes da tag**, para não versionar defeito conhecido.

**Item 1 — delimitador não pareado.** `ADR: "docs/adr/X.md'` resolve `docs/adr/X.md` em Go e
Node; o Python devolve vazio, porque `normalize_yaml_flat_value` só remove **par casado**.

**Item 2 — não era o que eu havia reportado.** Registrei como "parser YAML do Python não trata
lista inline". Medido: **nenhum dos três trata** — todos só entendem lista em bloco e, com
`agents: [a, b]`, caem no fallback de varrer subdiretórios. A divergência real está **no
fallback**: `_list_dirs` (`pypi/trackfw/commands/status.py`) **não ordena**, enquanto a irmã
`_list_files` no mesmo arquivo ordena, e Go/Node ordenam.

## Acceptance Criteria

- [ ] **AC1** — Delimitador não pareado: os 3 CLIs produzem **a mesma** saída para
      `ADR: "docs/adr/X.md'`. O Python passa a resolver `docs/adr/X.md`.
- [ ] **AC2** — A tabela de 8 entradas do PR #104 é reexecutada nos 3 CLIs e agora dá
      **idêntica nos 8 casos**, incluindo o caso 6. Saída literal no relatório.
- [ ] **AC3** — `_list_dirs` ordena. Fixture com subdiretórios criados fora de ordem alfabética
      → os 3 CLIs listam agentes na **mesma** ordem.
- [ ] **AC4** — `status` em modo `by_agent` **sem** `agents:` configurado (portanto via
      fallback): as 3 saídas **byte-idênticas**. Hoje divergem.
- [ ] **AC5** — Não regride: `status` flat e `by_agent` **com** `agents:` em bloco continuam
      byte-idênticos nos 3; repositório real continua em 749 bytes.
- [ ] **AC6** — `validate` verde nos 3 CLIs no repositório.
- [ ] **AC7** — Cenário de falsificação cobrindo **os dois** itens: fixture com delimitador não
      pareado e fixture `by_agent` **sem `agents:`** (subdiretórios fora de ordem alfabética).
      Sem essas fixtures os cenários não discriminam.
- [ ] **AC8** — `make build`, `make test`, `make lint`, `make parity` e `make quality` verdes.

## Negative Scope (fora do escopo — NÃO fazer)

- **Não tocar em Go nem em Node.** Nos dois itens eles concordam entre si e são a referência.
- **Não suportar lista YAML inline** (`agents: [a, b]`) em nenhum CLI. Os três a ignoram em
  silêncio — é defeito **consistente**, portanto não é problema de paridade, e resolvê-lo exige
  decisão de produto (suportar versus avisar). Fica na fila como item próprio.
- Não alterar o formato de saída do `status` — apenas a **ordem** dos agentes no fallback.
- Não alterar `extractRefPath` fora do ponto de remoção de delimitador.
- Não alterar `normalize_yaml_flat_value` de forma a afetar `parse_frontmatter` — o PR #104
  estreitou isso de propósito, e há teste de regressão. **Não reverter esse estreitamento.**
- Não alterar o status de nenhum ADR ou REQ do repositório.
- Não mexer em `pypi/build/lib/`; não adicionar dependência.

## Notas de implementação

| Item | Ponto exato |
|---|---|
| 1 | `pypi/trackfw/validator.py` — remoção de delimitador no caminho de `_extract_ref_path` |
| 2 | `pypi/trackfw/commands/status.py` — `_list_dirs` (~linha 37) |

**Atenção ao item 1:** o PR #104 estreitou deliberadamente o alcance, dando a `_extract_ref_path`
remoção **própria** de backtick para não afetar `parse_frontmatter` (que é usado por 10 call
sites). Há teste de regressão garantindo que o frontmatter **preserva** backtick. A mudança agora
deve seguir contida ao caminho da extração de referência — **não** voltar a mexer no helper
compartilhado.

Referência da tabela de 8 entradas: seção AC5 de
`docs/req/REQ-2026-08-02-backticks-em-campos-de-referencia-e-mensagem-de-sucesso-do-validate-no-python.md`.

As duas correções são **só no Python** → um único ML de implementação, mais barreira.

`check-gates-falsify.sh` leva mais de 2 min; rodar em background.

## Linked ADR

ADR: docs/adr/ADR-2026-08-02-python-alinha-delimitador-nao-pareado-e-ordenacao-do-fallback-de-agentes.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/wip/ROADMAP-2026-08-02-fechar-as-duas-divergencias-de-parsing-remanescentes-no-python.md
