---
status: Open
date: 2026-08-30
author: "trackfw_architect (Zeus)"
adr: ""
roadmap: "docs/roadmaps/wip/ROADMAP-2026-09-01-caminho-dentro-de-artefato-versionado-usa-sempre-barra.md"
---

# REQ: Caminho portável montado com separador do sistema vaza para dentro de artefato versionado

> Date: 2026-08-30 | Status: Open

## Motivation

Reportado por **@lourivalgarciajunior** na issue #216 (décimo primeiro achado), com dois pontos
medidos no Windows:

**1. O sync do `roadmap move`** grava o caminho com o separador nativo no frontmatter da REQ:

```
antes      roadmap: docs/roadmaps/backlog/ROADMAP-x.md
move wip   ✓ synced REQ-x.md → docs\roadmaps\wip\ROADMAP-x.md
depois     roadmap: docs\roadmaps\wip\ROADMAP-x.md
```

**2. `pypi/trackfw/commands/roadmap.py:609`** — `os.path.join(agent, basename)` grava
`zeus\ARQUIVO.md` no `.trackfw-log`.

No Windows resolve, porque o `os.Stat` aceita as duas grafias. **Em Linux não resolve** — e o
arquivo **vai para o git**. Basta alguém commitar no Windows e outro dar checkout no Linux para a
referência quebrar.

**A classe é a mesma do CRLF** (item 5 da issue) e do que corrigimos no PR #217: **conteúdo de
artefato versionado montado com convenção do sistema operacional**. Caminho dentro de arquivo não é
caminho de sistema de arquivos — é **dado portável**, e tem que ser sempre `/`.

## Acceptance Criteria

- [ ] **AC1** — Todo caminho **escrito dentro** de artefato versionado usa `/`, independentemente do
      SO: frontmatter (`adr:`, `roadmap:`, `req:`), `.trackfw-log`, e o que a varredura encontrar.
- [ ] **AC2** — **Varredura obrigatória**: enumerar todos os pontos, nos 3 runtimes, onde um caminho
      vira conteúdo de artefato. Corrigir a classe, não as duas instâncias relatadas. O relato já
      sugere isso; é o pedido dele e está certo.
- [ ] **AC3** — **Leitura tolerante:** caminho já gravado com `\` continua sendo resolvido, para não
      quebrar quem já tem o artefato sujo. Tolerar na leitura, normalizar na escrita.
- [ ] **AC4** — Falsificação nas duas direções: escrita produz `/` mesmo com separador nativo `\`;
      leitura resolve as duas grafias.
- [ ] **AC5** — Paridade nos 3 CLIs; gate falsificável — que precisa provar a escrita **sem** máquina
      Windows, provavelmente injetando o separador em vez de depender do SO.
- [ ] **AC6** — `make quality` exit 0 e CI verde.

## Negative Scope

- **Não** normalizar caminho de sistema de arquivos em uso interno — `filepath.Join` continua certo
  para abrir arquivo. O escopo é **conteúdo de artefato**.
- **Não** migrar artefato já gravado com `\`. A AC3 cobre a leitura.

## Observação

O relato pediu explicitamente a varredura, não só a correção dos dois pontos. É o mesmo pedido que
fizemos a nós mesmos no `newline=` do Python e no `lstat` — e nas duas vezes a varredura achou mais
do que o relato original.

## Linked ADR
<!-- Provável: caminho em conteúdo de artefato é sempre POSIX. -->
ADR:

## Blocked by ADRs
<!-- none -->

## Linked Roadmap
Roadmap:
