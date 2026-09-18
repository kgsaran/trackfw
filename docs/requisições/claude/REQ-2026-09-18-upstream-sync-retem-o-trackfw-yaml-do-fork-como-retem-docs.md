---
status: Open
date: 2026-09-18
author: "claude"
adr: "docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md"
roadmap: "docs/roadmaps/claude/wip/ROADMAP-2026-09-18-upstream-sync-retem-o-trackfw-yaml-do-fork-como-retem-docs.md"
---

# REQ: upstream-sync retém o trackfw.yaml do fork como retém docs

> Date: 2026-09-18 | Status: Open

## Motivation

O `trackfw.yaml` é a configuração da governança **deste** fork — `req_dir: docs/requisições`,
`roadmap_namespacing: by_agent`, três agentes, `governance_mode: strict`. A `ADR-2026-08-29` decide
que governança é local, mas o `upstream-sync.sh` só retinha `docs/` e `vault/`: o arquivo contava como
produto. Enquanto o upstream não o tocava, ninguém percebeu. O #393 dele (2026-09-18) acrescentou
`lenient_until`, as linhas colidiram, e o sync abortou.

Decisão do usuário em 2026-09-18: **reter sempre o nosso**, e o sync avisar com o diff dele quando o
upstream mudar o arquivo.

## Acceptance Criteria

- [x] **AC1** — o `upstream-sync.sh` retém o `trackfw.yaml` da base, inclusive em conflito, e a
      retenção é provada por efeito junto com `docs/` e `vault/`.
- [x] **AC2** — quando o upstream muda o arquivo, o sync imprime o diff dele.
- [x] **AC3** — a falsificação ganha o merge real do #393 como terceiro caso, e um sync sem a
      retenção reprova nele.
- [ ] **AC4** — o sync do #393 passa com o `validate` igual antes e depois.

## Evidência — 2026-09-18

**O defeito, medido.** `upstream-sync.sh` sobre `9651f905` (#393 do upstream): `trackfw.yaml`
`Unmerged`, `conflito de PRODUTO remanescente`, árvore devolvida. O upstream acrescentou
`lenient_until: "2027-12-31"` abaixo do `governance_mode: lenient` dele; o nosso tem
`governance_mode: strict` e o trecho diverge.

| caso da falsificação | sync novo | controle: sync sem a retenção |
|---|---|---|
| `bfeea12...4f0ad33` (governança pesada) | OK | — |
| `01086b5...6b3ba49` (produto puro) | OK | — |
| `65cb024b...9651f905` (**o #393, trackfw.yaml em conflito**) | OK — retido 20, trazido 22, `trackfw.yaml` idêntico à base | **rc=1** — `trackfw.yaml Unmerged`, `RETENÇÃO NÃO PROVADA` |

**Frase por teste:** o terceiro caso afirma que o sync retém o `trackfw.yaml` do fork mesmo quando o
upstream o muda em linhas que colidem com as nossas; o controle afirma que, sem a retenção, o mesmo
merge não passa.

## Escopo negativo

- **Não** aplica o `lenient_until` nem nenhuma outra chave do `trackfw.yaml` dele: somos `strict`.

## Linked ADR
ADR: docs/adr/ADR-2026-08-29-adotar-upstream-como-base.md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap: docs/roadmaps/claude/wip/ROADMAP-2026-09-18-upstream-sync-retem-o-trackfw-yaml-do-fork-como-retem-docs.md
