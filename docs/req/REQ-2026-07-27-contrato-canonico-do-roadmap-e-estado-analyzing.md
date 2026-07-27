---
status: Open
date: 2026-07-27
author: "zeus"
adr: "docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md"
roadmap: "docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md"
---

# REQ: Contrato canônico do roadmap e estado analyzing

> Date: 2026-07-27 | Status: Open
| Linear Issue:
| Jira Issue:

## Motivation

O trackfw possui dois contratos contraditórios para roadmaps:

1. `trackfw roadmap new` gera o formato canônico com frontmatter (`status`, `date`, `req`, `squad`),
   mas o slash-command `/trackfw:roadmap`, instalado pelos três CLIs, orienta o agente a criar um
   arquivo sem frontmatter. O artefato nasce invisível ou inconsistente para regras que dependem de
   metadados estruturados.
2. O estado `analyzing` é criado pelo scaffold e reconhecido pelo validator, pelo status e pelos
   resolvers de estado, mas os geradores/movimentadores dos três CLIs o rejeitam. A própria skill
   trackfw exige mover um roadmap de `backlog/` para `analyzing/` durante análise, porém o comando
   oficial não consegue cumprir esse fluxo.

Os dois defeitos pertencem ao mesmo limite de domínio: **o contrato de criação e transição de estado
do roadmap não é único entre interfaces humanas, slash-commands e CLIs**. Corrigi-los juntos permite
uma prova de ciclo completo:

`/trackfw:roadmap` → roadmap canônico em `backlog/` → `roadmap move ... analyzing` → validação verde.

## Acceptance Criteria

- [ ] O slash-command `/trackfw:roadmap` gerado por Go, Node.js e Python exige frontmatter canônico
      byte-equivalente ao template de `trackfw roadmap new`: `status`, `date`, `req` e `squad`.
- [ ] O slash-command preenche `req:` com caminho relativo completo para a REQ selecionada e cria o
      arquivo em `docs/roadmaps/backlog/`.
- [ ] O estado `analyzing` é aceito por `roadmap move` nos três CLIs, em layout flat e `by_agent`.
- [ ] Ao mover para `analyzing`, pasta, frontmatter e header ficam sincronizados e o transition log
      registra a mudança no mesmo formato dos demais estados.
- [ ] `roadmap list` e `roadmap show` encontram roadmaps em `analyzing/` nos três CLIs.
- [ ] A documentação de comandos e o contrato de paridade declaram `analyzing` como estado válido.
- [ ] Testes negativos provam, antes da correção, que o slash-command nasce sem frontmatter e que
      `roadmap move ... analyzing` é rejeitado nos três runtimes.
- [ ] Gate cross-CLI compara o slash-command gerado pelos três runtimes e reprova drift de conteúdo.
- [ ] Prova de ciclo completo passa nos três CLIs sem warning `folder_status`.
- [ ] `make quality` e `trackfw validate` passam sem violations ou warnings.

## Scope

### Included

- Template do slash-command `/trackfw:roadmap` nos geradores `init` dos três CLIs.
- Estados válidos, busca, listagem, movimentação e log de roadmap nos três CLIs.
- Testes unitários, integração, paridade e falsificação necessários.
- Documentação do contrato canônico e do estado `analyzing`.

### Excluded

- Alterar a semântica de `stale_wip`.
- Migrar roadmaps históricos que já estejam sem frontmatter.
- Criar novos estados além de `analyzing`.
- Redesenhar o modelo Kanban ou o namespacing `by_agent`.
- Alterar `req move` ou o ciclo de vida da REQ.

## Linked ADR

ADR: `docs/adr/ADR-2026-07-26-principios-de-design-de-gates-verificaveis.md`

## Blocked by ADRs

<!-- none -->

## Linked Roadmap

Roadmap: `docs/roadmaps/wip/ROADMAP-2026-07-27-contrato-canonico-do-roadmap-e-estado-analyzing.md`
