# `roadmap:` no frontmatter da REQ NÃO satisfaz a regra de Roadmap linkado

> Data: 2026-08-02 | Autor: Zeus | Domínio: validator / autoria manual de artefatos

## Sintoma

REQ escrita à mão, com `roadmap: "docs/roadmaps/backlog/ROADMAP-....md"` no frontmatter apontando
para um arquivo que **existe**. O `trackfw validate` mesmo assim reprova:

```
✗ req "REQ-....md" has no linked Roadmap
```

Confuso porque o campo está lá, o caminho está certo, e `ref_targets_exist` — que também lê
frontmatter — não reclama de nada.

## Causa

São **dois mecanismos diferentes** lendo a mesma relação:

- `ref_targets_exist` valida que o alvo de uma referência **existe**, e enxerga o frontmatter.
- A regra de Roadmap linkado procura um **link field no corpo** do documento: uma linha começando
  com `Roadmap:` (configurável em `link_fields_roadmap`, default `["Roadmap:"]`).

O frontmatter `roadmap:` **não** conta como link field. As duas grafias coexistem por razões
históricas e não são intercambiáveis: uma é metadado, a outra é a aresta da cadeia.

## Por que não aparece no fluxo normal

`trackfw req new` gera o esqueleto com as seções `## Linked ADR`, `## Blocked by ADRs` e
`## Linked Roadmap` já presentes, então quem usa o gerador nunca encontra isso. O problema é
específico de **REQ escrita à mão** — que é o caminho de qualquer agente que use `Write` direto
em vez do CLI.

Vale para o `ADR:` do mesmo jeito: `adr:` no frontmatter não substitui a linha `ADR:` no corpo.

## Correção

Acrescentar ao corpo da REQ, tipicamente ao final:

```markdown
## Linked ADR

ADR: docs/adr/ADR-....md

## Blocked by ADRs
<!-- none -->

## Linked Roadmap

Roadmap: docs/roadmaps/backlog/ROADMAP-....md
```

Manter também o frontmatter — os dois têm consumidores distintos (o dashboard e o `chain` usam o
frontmatter).

**Atenção ao `## Blocked by ADRs`**: essa seção é para ADR que *bloqueia* a REQ, não para
traceability. Preenchê-la com o ADR pareado normal faz `blocked_by_draft_adr` disparar se o ADR
não estiver `Accepted` — o que é correto, mas surpreende quem a usou como sinônimo de "ADR
relacionado". Deixe `<!-- none -->` quando nada estiver bloqueando.

## Regra prática

Escreveu artefato à mão? Rode `trackfw validate` **antes** de commitar, e compare a estrutura com
o artefato irmão mais recente do mesmo tipo — não com o template mental.
